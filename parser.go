package xmlstreamer

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"

	"github.com/orisano/gosax"
	"github.com/wilkmaciej/xpath"
)

// Parser provides streaming XML parsing with XPath support.
type Parser struct {
	ctx         context.Context
	reader      io.Reader
	streamNames map[string]bool // Optional: specific element names to stream
	bufferSize  int
	once        sync.Once
	ch          chan *XMLElement
}

// NewParser creates a new XML parser
// streamNames: specific element names to stream (pass nil or empty slice to stream nothing)
// bufferSize: channel buffer size for streaming (pass 0 to use default of 8)
func NewParser(ctx context.Context, reader io.Reader, streamNames []string, bufferSize int) *Parser {
	if bufferSize <= 0 {
		bufferSize = 8
	}

	p := &Parser{
		ctx:        ctx,
		reader:     reader,
		bufferSize: bufferSize,
	}

	if len(streamNames) > 0 {
		p.streamNames = make(map[string]bool)
		for _, name := range streamNames {
			p.streamNames[name] = true
		}
	}

	return p
}

// Stream returns a channel of XMLElements as they are parsed.
// It is safe to call multiple times — subsequent calls return the same channel.
func (p *Parser) Stream() <-chan *XMLElement {
	p.once.Do(func() {
		p.ch = make(chan *XMLElement, p.bufferSize)
		go func() {
			defer close(p.ch)
			p.parse(p.ch)
		}()
	})
	return p.ch
}

type parseState struct {
	stack []*XMLElement
	depth int
}

func (p *Parser) parse(ch chan<- *XMLElement) {
	state := &parseState{
		stack: make([]*XMLElement, 0, 32),
	}

	r := gosax.NewReaderSize(p.reader, 1024*1024*64)

	for {
		e, err := r.Event()
		if err != nil || e.Type() == gosax.EventEOF || p.ctx.Err() != nil {
			break
		}

		switch e.Type() {
		case gosax.EventStart:
			name, attrs := gosax.Name(e.Bytes)
			// Only extract namespaces if xmlns is present (performance optimization)
			var elementNamespaces map[string]string
			if len(attrs) > 0 && bytes.Contains(attrs, []byte("xmlns")) {
				elementNamespaces = p.extractNamespaces(attrs)
			}
			p.handleStartElement(state, ch, name, attrs, e.Bytes, elementNamespaces)

		case gosax.EventEnd:
			p.handleEndElement(state, ch)

		case gosax.EventText:
			if len(state.stack) > 0 && len(e.Bytes) > 0 {
				parent := state.stack[len(state.stack)-1]
				node := getContentNodeFromPool()
				// Store offsets into parent's rawContent buffer
				node.start = len(parent.rawContent)
				parent.rawContent = append(parent.rawContent, e.Bytes...)
				node.end = len(parent.rawContent)
				node.nodeType = xpath.TextNode
				node.parent = parent
				node.siblingIndex = len(parent.children)
				parent.children = append(parent.children, node)
			}

		case gosax.EventCData:
			if len(state.stack) > 0 {
				// Strip <![CDATA[ prefix and ]]> suffix
				content := e.Bytes
				if len(content) > 12 { // len("<![CDATA[]]>") = 12
					content = content[9 : len(content)-3] // Remove "<![CDATA[" and "]]>"
					if len(content) > 0 {
						parent := state.stack[len(state.stack)-1]
						node := getContentNodeFromPool()
						// Store offsets into parent's rawContent buffer
						node.start = len(parent.rawContent)
						parent.rawContent = append(parent.rawContent, content...)
						node.end = len(parent.rawContent)
						node.nodeType = xpath.TextNode
						node.parent = parent
						node.siblingIndex = len(parent.children)
						parent.children = append(parent.children, node)
					}
				}
			}

		case gosax.EventComment:
			if len(state.stack) > 0 {
				// Strip <!-- prefix and --> suffix
				content := e.Bytes
				if len(content) > 7 { // len("<!---->") = 7
					content = content[4 : len(content)-3] // Remove "<!--" and "-->"
					parent := state.stack[len(state.stack)-1]
					node := getContentNodeFromPool()
					// Store offsets into parent's rawContent buffer
					node.start = len(parent.rawContent)
					parent.rawContent = append(parent.rawContent, content...)
					node.end = len(parent.rawContent)
					node.nodeType = xpath.CommentNode
					node.parent = parent
					node.siblingIndex = len(parent.children)
					parent.children = append(parent.children, node)
				}
			}
		}
	}
}

func (p *Parser) handleStartElement(state *parseState, ch chan<- *XMLElement, name []byte, attrs []byte, fullTag []byte, elementNamespaces map[string]string) {
	nameStr := string(name)

	// Parse element name for namespace support
	localName := nameStr
	prefix := ""
	if idx := strings.IndexByte(nameStr, ':'); idx != -1 {
		prefix = nameStr[:idx]
		localName = nameStr[idx+1:]
	}

	// Build namespace context for this element
	// Optimization: only copy parent context if we have new namespace declarations
	var nsContext map[string]string
	var parentNS map[string]string
	if len(state.stack) > 0 {
		parentNS = state.stack[len(state.stack)-1].namespaces
	}
	if len(elementNamespaces) > 0 {
		// We have new declarations, need to copy and merge
		if parentNS != nil {
			nsContext = make(map[string]string, len(parentNS)+len(elementNamespaces))
			for k, v := range parentNS {
				nsContext[k] = v
			}
		} else {
			nsContext = make(map[string]string, len(elementNamespaces))
		}
		for k, v := range elementNamespaces {
			nsContext[k] = v
		}
	} else if parentNS != nil {
		// No new declarations, reuse parent context
		nsContext = parentNS
	}

	// Resolve namespace URI for this element
	namespaceURI := ""
	if nsContext != nil {
		if prefix != "" {
			namespaceURI = nsContext[prefix]
		} else {
			namespaceURI = nsContext[""] // Default namespace
		}
	}

	// Get element from pool (already cleared by returnElementToPool)
	elem := getElementFromPool()
	elem.Name = nameStr
	elem.localName = localName
	elem.prefix = prefix
	elem.namespaceURI = namespaceURI
	elem.namespaces = nsContext

	// Parse attributes only if they exist
	if len(attrs) > 0 {
		parseAttributes(attrs, elem)
	}

	// Set parent relationship
	if len(state.stack) > 0 {
		parent := state.stack[len(state.stack)-1]
		elem.parent = parent
		elem.siblingIndex = len(parent.children)
		parent.children = append(parent.children, elem)
	}

	// Check if self-closing tag
	isSelfClosing := len(fullTag) >= 2 && fullTag[len(fullTag)-2] == '/' && fullTag[len(fullTag)-1] == '>'

	if isSelfClosing {
		// Handle self-closing tag
		p.checkAndStreamElement(ch, elem)
	} else {
		// Push to stack
		state.stack = append(state.stack, elem)
		state.depth++
	}
}

func (p *Parser) handleEndElement(state *parseState, ch chan<- *XMLElement) {
	if len(state.stack) == 0 {
		return
	}

	// Pop element from stack
	elem := state.stack[len(state.stack)-1]
	state.stack = state.stack[:len(state.stack)-1]

	// Check if we should stream this element
	p.checkAndStreamElement(ch, elem)

	state.depth--
}

func (p *Parser) checkAndStreamElement(ch chan<- *XMLElement, elem *XMLElement) {
	shouldStream := false

	// Check by name if streamNames is set
	if len(p.streamNames) > 0 {
		if p.streamNames[elem.Name] {
			shouldStream = true
		}
	}

	if shouldStream {
		// Detach from parent for streaming
		elem.parent = nil
		// Parent pointers for children are already set correctly during parsing
		ch <- elem
	}
	// Non-streamed elements are not automatically returned to pool.
	// They remain in memory as children of their parent and will be
	// returned when the parent is released via Release().
}

// parseAttributes parses attribute bytes and populates the element's attributes
func parseAttributes(attrs []byte, elem *XMLElement) {
	// Count attributes first for better allocation
	attrCount := 0
	for i := 0; i < len(attrs); i++ {
		if attrs[i] == '=' {
			attrCount++
		}
	}

	if attrCount == 0 {
		return
	}

	// Reuse existing slice if it has enough capacity, otherwise allocate
	if cap(elem.Attributes) >= attrCount {
		elem.Attributes = elem.Attributes[:0]
	} else {
		elem.Attributes = make([]XMLAttribute, 0, attrCount)
	}

	// Simple attribute parser
	i := 0
	for i < len(attrs) {
		// Skip whitespace
		for i < len(attrs) && (attrs[i] == ' ' || attrs[i] == '\t' || attrs[i] == '\n' || attrs[i] == '\r') {
			i++
		}
		if i >= len(attrs) {
			break
		}

		// Find attribute name
		nameStart := i
		for i < len(attrs) && attrs[i] != '=' {
			i++
		}
		if i >= len(attrs) {
			break
		}
		name := string(bytes.TrimSpace(attrs[nameStart:i]))

		// Skip '='
		i++

		// Skip whitespace
		for i < len(attrs) && (attrs[i] == ' ' || attrs[i] == '\t') {
			i++
		}
		if i >= len(attrs) {
			break
		}

		// Find attribute value (quoted)
		quote := attrs[i]
		if quote != '"' && quote != '\'' {
			break
		}
		i++
		valueStart := i
		for i < len(attrs) && attrs[i] != quote {
			i++
		}
		value := string(attrs[valueStart:i])
		i++ // Skip closing quote

		// Store attribute inline (no allocation, stored in slice backing array)
		elem.Attributes = append(elem.Attributes, XMLAttribute{Name: name, Value: value})
	}
}

// extractNamespaces scans attributes for xmlns declarations and returns them
func (p *Parser) extractNamespaces(attrs []byte) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	var namespaces map[string]string

	i := 0
	for i < len(attrs) {
		// Skip whitespace
		for i < len(attrs) && (attrs[i] == ' ' || attrs[i] == '\t' || attrs[i] == '\n' || attrs[i] == '\r') {
			i++
		}
		if i >= len(attrs) {
			break
		}

		// Find attribute name
		nameStart := i
		for i < len(attrs) && attrs[i] != '=' {
			i++
		}
		if i >= len(attrs) {
			break
		}
		nameBytes := bytes.TrimSpace(attrs[nameStart:i])

		// Quick check: skip if not a namespace declaration (early bailout for performance)
		if len(nameBytes) < 5 || (nameBytes[0] != 'x' && nameBytes[0] != 'X') {
			// Skip '=' and value quickly
			i++
			for i < len(attrs) && (attrs[i] == ' ' || attrs[i] == '\t') {
				i++
			}
			if i < len(attrs) && (attrs[i] == '"' || attrs[i] == '\'') {
				quote := attrs[i]
				i++
				for i < len(attrs) && attrs[i] != quote {
					i++
				}
				i++
			}
			continue
		}

		name := string(nameBytes)

		// Skip '='
		i++

		// Skip whitespace
		for i < len(attrs) && (attrs[i] == ' ' || attrs[i] == '\t') {
			i++
		}
		if i >= len(attrs) {
			break
		}

		// Find attribute value (quoted)
		quote := attrs[i]
		if quote != '"' && quote != '\'' {
			break
		}
		i++
		valueStart := i
		for i < len(attrs) && attrs[i] != quote {
			i++
		}
		value := string(attrs[valueStart:i])
		i++ // Skip closing quote

		// Check if it's a namespace declaration
		if name == "xmlns" {
			// Default namespace - store with empty prefix
			if namespaces == nil {
				namespaces = make(map[string]string, 2)
			}
			namespaces[""] = value
		} else if strings.HasPrefix(name, "xmlns:") {
			// Prefixed namespace
			if namespaces == nil {
				namespaces = make(map[string]string, 2)
			}
			prefix := name[6:] // Remove "xmlns:" prefix
			namespaces[prefix] = value
		}
	}
	if namespaces == nil {
		return nil
	}
	return namespaces
}
