package xmlstreamer

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/wilkmaciej/xpath"
)

// XMLNode is the interface implemented by all XML node types
type XMLNode interface {
	Parent() *XMLElement
	InnerText() string
	getSiblingIndex() int
}

// XMLContentNode represents a text or comment node in the XML tree
// Content is stored as offsets into parent's rawContent buffer for zero-copy access
type XMLContentNode struct {
	start        int // start offset in parent.rawContent
	end          int // end offset in parent.rawContent
	nodeType     xpath.NodeType
	parent       *XMLElement
	siblingIndex int // index within parent's children slice for O(1) sibling navigation
}

// Parent returns the parent element
func (c *XMLContentNode) Parent() *XMLElement {
	return c.parent
}

// InnerText returns the content using zero-copy string conversion
func (c *XMLContentNode) InnerText() string {
	if c.parent == nil || c.start >= c.end {
		return ""
	}
	// Zero-copy conversion - safe because rawContent is not modified after parsing
	return unsafe.String(&c.parent.rawContent[c.start], c.end-c.start)
}

// getSiblingIndex returns the index within parent's children
func (c *XMLContentNode) getSiblingIndex() int {
	return c.siblingIndex
}

// XMLElement represents an XML element with XPath query capabilities
type XMLElement struct {
	Name string

	// Internal fields for XPath navigation
	children     []XMLNode
	parent       *XMLElement
	Attributes   []XMLAttribute
	localName    string
	prefix       string
	namespaceURI string            // The resolved namespace URI for this element
	namespaces   map[string]string // prefix -> URI mapping for this element's scope
	siblingIndex int               // index within parent's children slice for O(1) sibling navigation
	rawContent   []byte            // Raw byte buffer for text content (children reference slices of this)
}

// XMLAttribute represents an XML attribute
type XMLAttribute struct {
	Name  string
	Value string
}

// Parent returns the parent element
func (e *XMLElement) Parent() *XMLElement {
	return e.parent
}

// getSiblingIndex returns the index within parent's children
func (e *XMLElement) getSiblingIndex() int {
	return e.siblingIndex
}

// InnerText returns the concatenated text content of this element and all descendants
func (e *XMLElement) InnerText() string {
	if len(e.children) == 0 {
		return ""
	}

	// Fast path: if no nested element children, rawContent is the complete text
	hasElementChild := false
	for _, child := range e.children {
		if _, ok := child.(*XMLElement); ok {
			hasElementChild = true
			break
		}
	}
	if !hasElementChild {
		// Zero-copy: rawContent contains all text content for this element
		return unsafe.String(unsafe.SliceData(e.rawContent), len(e.rawContent))
	}

	// Slow path: need to recursively collect text from nested elements
	var sb strings.Builder
	e.collectText(&sb)
	return sb.String()
}

// collectText recursively collects text content from this element and descendants
func (e *XMLElement) collectText(sb *strings.Builder) {
	for _, child := range e.children {
		switch node := child.(type) {
		case *XMLContentNode:
			if node.nodeType == xpath.TextNode && node.parent != nil && node.start < node.end {
				sb.Write(node.parent.rawContent[node.start:node.end])
			}
			// Skip comment nodes in text collection
		case *XMLElement:
			node.collectText(sb)
		}
	}
}

// Evaluate evaluates an XPath expression and returns the result.
// The result type depends on the expression:
//   - Node-set expressions return []any containing *XMLElement, *XMLContentNode, or *XMLAttribute
//   - String functions return string
//   - Numeric functions (count, sum, etc.) return float64
//   - Boolean expressions return bool
func (e *XMLElement) Evaluate(exp *xpath.Expr) any {
	nav := &elementNavigator{currNode: e, currElement: e, root: e, attributeIndex: -1}
	result := exp.Evaluate(nav)

	// Convert NodeIterator to []any for consistency
	if iter, ok := result.(*xpath.NodeIterator); ok {
		// Preallocate for common case of 1 result
		elements := make([]any, 0, 1)
		for iter.MoveNext() {
			if nav, ok := iter.Current().(*elementNavigator); ok {
				if nav.attributeIndex != -1 {
					elements = append(elements, &nav.currElement.Attributes[nav.attributeIndex])
				} else {
					elements = append(elements, nav.currNode)
				}
			}
		}
		return elements
	}

	return result
}

// Release returns this element and all its children back to the pool for reuse.
// IMPORTANT: After calling Release(), you must not use this element or any of its
// children anymore, as they may be reused by the parser. Only call this when you're
// completely done processing the element.
// This is optional - if not called, the GC will clean up normally (just slower).
func (e *XMLElement) Release() {
	returnElementToPool(e)
}

// xmlElementPool is a pool for reusing XMLElement allocations
var xmlElementPool = sync.Pool{
	New: func() any {
		return &XMLElement{
			children:   make([]XMLNode, 0, 4),
			rawContent: make([]byte, 0, 128), // Pre-allocate typical content size
		}
	},
}

// xmlContentNodePool is a pool for reusing XMLContentNode allocations
var xmlContentNodePool = sync.Pool{
	New: func() any {
		return &XMLContentNode{}
	},
}

// getContentNodeFromPool gets a content node from the pool
func getContentNodeFromPool() *XMLContentNode {
	return xmlContentNodePool.Get().(*XMLContentNode)
}

// returnContentNodeToPool returns a content node to the pool
// Note: we don't clear fields because all fields are always overwritten when reused
func returnContentNodeToPool(node *XMLContentNode) {
	xmlContentNodePool.Put(node)
}

// returnElementToPool returns an element to the pool for reuse.
// This is called internally for non-streamed elements and can be called
// via element.Release() for streamed elements.
// Uses an iterative approach to avoid recursion overhead.
func returnElementToPool(elem *XMLElement) {
	stack := make([]*XMLElement, 0, 16)
	stack = append(stack, elem)

	for len(stack) > 0 {
		// Pop from stack
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Process children - add child elements to stack, return content nodes to pool
		for _, child := range current.children {
			switch c := child.(type) {
			case *XMLElement:
				stack = append(stack, c)
			case *XMLContentNode:
				returnContentNodeToPool(c)
			}
		}

		// Clear element fields - keep backing slices for reuse
		current.children = current.children[:0]
		current.parent = nil
		current.Attributes = current.Attributes[:0]
		current.namespaces = nil
		current.siblingIndex = 0
		current.rawContent = current.rawContent[:0] // Keep backing array
		xmlElementPool.Put(current)
	}
}

// getElementFromPool gets a fresh element from the pool
func getElementFromPool() *XMLElement {
	return xmlElementPool.Get().(*XMLElement)
}
