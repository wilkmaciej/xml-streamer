package xmlstreamer

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/wilkmaciej/xpath"
)

// =============================================================================
// TEST UTILITIES
// =============================================================================

func parseAll(t *testing.T, xml string, streamNames []string) []*XMLElement {
	t.Helper()
	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), streamNames, 10)
	var elements []*XMLElement
	for elem := range parser.Stream() {
		elements = append(elements, elem)
	}
	return elements
}

func parseOne(t *testing.T, xml string, streamName string) *XMLElement {
	t.Helper()
	elements := parseAll(t, xml, []string{streamName})
	if len(elements) == 0 {
		t.Fatalf("expected at least one element, got none")
	}
	return elements[0]
}

// =============================================================================
// BASIC PARSING TESTS
// =============================================================================

func TestBasicElement(t *testing.T) {
	xml := `<root><item>hello</item></root>`
	elem := parseOne(t, xml, "item")

	if elem.Name != "item" {
		t.Errorf("expected name 'item', got %q", elem.Name)
	}
	if elem.InnerText() != "hello" {
		t.Errorf("expected inner text 'hello', got %q", elem.InnerText())
	}
}

func TestEmptyElement(t *testing.T) {
	xml := `<root><item></item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "" {
		t.Errorf("expected empty inner text, got %q", elem.InnerText())
	}
}

func TestSelfClosingElement(t *testing.T) {
	xml := `<root><item/></root>`
	elem := parseOne(t, xml, "item")

	if elem.Name != "item" {
		t.Errorf("expected name 'item', got %q", elem.Name)
	}
}

func TestSelfClosingWithSpace(t *testing.T) {
	xml := `<root><item /></root>`
	elem := parseOne(t, xml, "item")

	if elem.Name != "item" {
		t.Errorf("expected name 'item', got %q", elem.Name)
	}
}

func TestMultipleElements(t *testing.T) {
	xml := `<root><item>one</item><item>two</item><item>three</item></root>`
	elements := parseAll(t, xml, []string{"item"})

	if len(elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elements))
	}

	expected := []string{"one", "two", "three"}
	for i, elem := range elements {
		if elem.InnerText() != expected[i] {
			t.Errorf("element %d: expected %q, got %q", i, expected[i], elem.InnerText())
		}
	}
}

func TestNestedElements(t *testing.T) {
	xml := `<root><parent><child>nested</child></parent></root>`
	elem := parseOne(t, xml, "parent")

	if len(elem.children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(elem.children))
	}
	child, ok := elem.children[0].(*XMLElement)
	if !ok {
		t.Fatalf("expected *XMLElement, got %T", elem.children[0])
	}
	if child.Name != "child" {
		t.Errorf("expected child name 'child', got %q", child.Name)
	}
}

func TestDeeplyNested(t *testing.T) {
	xml := `<root><a><b><c><d><e>deep</e></d></c></b></a></root>`
	elem := parseOne(t, xml, "a")

	// Navigate down through children
	names := []string{"a", "b", "c", "d", "e"}
	current := elem
	for i, name := range names {
		if current == nil {
			t.Fatalf("expected element at depth %d", i)
		}
		if current.Name != name {
			t.Errorf("depth %d: expected %q, got %q", i, name, current.Name)
		}
		// Get first element child
		var next *XMLElement
		for _, child := range current.children {
			if e, ok := child.(*XMLElement); ok {
				next = e
				break
			}
		}
		current = next
	}
}

// =============================================================================
// ATTRIBUTE TESTS
// =============================================================================

func TestSingleAttribute(t *testing.T) {
	xml := `<root><item id="123">text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(elem.Attributes))
	}
	if elem.Attributes[0].Name != "id" {
		t.Errorf("expected attribute name 'id', got %q", elem.Attributes[0].Name)
	}
	if elem.Attributes[0].Value != "123" {
		t.Errorf("expected attribute value '123', got %q", elem.Attributes[0].Value)
	}
}

func TestMultipleAttributes(t *testing.T) {
	xml := `<root><item id="1" name="test" enabled="true">text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(elem.Attributes))
	}

	attrs := make(map[string]string)
	for _, attr := range elem.Attributes {
		attrs[attr.Name] = attr.Value
	}

	if attrs["id"] != "1" {
		t.Errorf("expected id='1', got %q", attrs["id"])
	}
	if attrs["name"] != "test" {
		t.Errorf("expected name='test', got %q", attrs["name"])
	}
	if attrs["enabled"] != "true" {
		t.Errorf("expected enabled='true', got %q", attrs["enabled"])
	}
}

func TestAttributeWithSingleQuotes(t *testing.T) {
	xml := `<root><item name='single'>text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(elem.Attributes))
	}
	if elem.Attributes[0].Value != "single" {
		t.Errorf("expected 'single', got %q", elem.Attributes[0].Value)
	}
}

func TestAttributeWithSpaces(t *testing.T) {
	xml := `<root><item name = "spaced" id= "1" class ="test">text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(elem.Attributes))
	}
}

func TestAttributeEmptyValue(t *testing.T) {
	xml := `<root><item name="">text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(elem.Attributes))
	}
	if elem.Attributes[0].Value != "" {
		t.Errorf("expected empty value, got %q", elem.Attributes[0].Value)
	}
}

func TestAttributeOnSelfClosing(t *testing.T) {
	xml := `<root><item id="123"/></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(elem.Attributes))
	}
	if elem.Attributes[0].Value != "123" {
		t.Errorf("expected '123', got %q", elem.Attributes[0].Value)
	}
}

// =============================================================================
// CDATA TESTS
// =============================================================================

func TestCDATABasic(t *testing.T) {
	xml := `<root><item><![CDATA[raw content]]></item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "raw content" {
		t.Errorf("expected 'raw content', got %q", elem.InnerText())
	}
}

func TestCDATAWithSpecialChars(t *testing.T) {
	xml := `<root><item><![CDATA[<script>alert('xss')</script>]]></item></root>`
	elem := parseOne(t, xml, "item")

	expected := `<script>alert('xss')</script>`
	if elem.InnerText() != expected {
		t.Errorf("expected %q, got %q", expected, elem.InnerText())
	}
}

func TestCDATAWithAmpersand(t *testing.T) {
	xml := `<root><item><![CDATA[Tom & Jerry]]></item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "Tom & Jerry" {
		t.Errorf("expected 'Tom & Jerry', got %q", elem.InnerText())
	}
}

func TestCDATAEmpty(t *testing.T) {
	xml := `<root><item><![CDATA[]]></item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "" {
		t.Errorf("expected empty, got %q", elem.InnerText())
	}
}

func TestCDATAWithNewlines(t *testing.T) {
	xml := "<root><item><![CDATA[line1\nline2\nline3]]></item></root>"
	elem := parseOne(t, xml, "item")

	expected := "line1\nline2\nline3"
	if elem.InnerText() != expected {
		t.Errorf("expected %q, got %q", expected, elem.InnerText())
	}
}

// =============================================================================
// ENTITY TESTS (XML predefined entities)
// Note: These tests document current behavior - entity decoding may not be implemented
// =============================================================================

func TestEntityLessThan(t *testing.T) {
	xml := `<root><item>&lt;tag&gt;</item></root>`
	elem := parseOne(t, xml, "item")

	// Document current behavior (entities may not be decoded)
	text := elem.InnerText()
	t.Logf("Entity &lt;/&gt; result: %q", text)
	// Expected if decoded: "<tag>"
	// Current behavior may be: "&lt;tag&gt;"
}

func TestEntityAmpersand(t *testing.T) {
	xml := `<root><item>Tom &amp; Jerry</item></root>`
	elem := parseOne(t, xml, "item")

	text := elem.InnerText()
	t.Logf("Entity &amp; result: %q", text)
	// Expected if decoded: "Tom & Jerry"
}

func TestEntityQuotes(t *testing.T) {
	xml := `<root><item>&quot;quoted&quot; and &apos;apostrophe&apos;</item></root>`
	elem := parseOne(t, xml, "item")

	text := elem.InnerText()
	t.Logf("Entity quotes result: %q", text)
	// Expected if decoded: `"quoted" and 'apostrophe'`
}

func TestNumericEntityDecimal(t *testing.T) {
	xml := `<root><item>&#65;&#66;&#67;</item></root>`
	elem := parseOne(t, xml, "item")

	text := elem.InnerText()
	t.Logf("Numeric decimal entity result: %q", text)
	// Expected if decoded: "ABC"
}

func TestNumericEntityHex(t *testing.T) {
	xml := `<root><item>&#x41;&#x42;&#x43;</item></root>`
	elem := parseOne(t, xml, "item")

	text := elem.InnerText()
	t.Logf("Numeric hex entity result: %q", text)
	// Expected if decoded: "ABC"
}

func TestEntityInAttribute(t *testing.T) {
	xml := `<root><item name="&lt;value&gt;">text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) > 0 {
		t.Logf("Entity in attribute result: %q", elem.Attributes[0].Value)
		// Expected if decoded: "<value>"
	}
}

// =============================================================================
// NAMESPACE TESTS
// =============================================================================

func TestDefaultNamespace(t *testing.T) {
	xml := `<root xmlns="http://example.com"><item>text</item></root>`
	elem := parseOne(t, xml, "item")

	if elem == nil {
		t.Fatal("expected element")
	}
	if elem.InnerText() != "text" {
		t.Errorf("expected 'text', got %q", elem.InnerText())
	}
}

func TestPrefixedNamespace(t *testing.T) {
	xml := `<ns:root xmlns:ns="http://example.com"><ns:item>text</ns:item></ns:root>`
	elem := parseOne(t, xml, "ns:item")

	if elem == nil {
		t.Fatal("expected element")
	}
	if elem.Name != "ns:item" {
		t.Errorf("expected name 'ns:item', got %q", elem.Name)
	}
}

func TestMultipleNamespaces(t *testing.T) {
	xml := `<root xmlns="http://default.com" xmlns:a="http://a.com" xmlns:b="http://b.com">
		<a:item>A</a:item>
		<b:item>B</b:item>
	</root>`
	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"a:item", "b:item"}, 10)

	count := 0
	for elem := range parser.Stream() {
		if elem.Name != "a:item" && elem.Name != "b:item" {
			t.Errorf("unexpected element: %s", elem.Name)
		}
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 elements, got %d", count)
	}
}

func TestNamespaceInheritance(t *testing.T) {
	xml := `<root xmlns:ns="http://example.com">
		<parent>
			<ns:child>inherited</ns:child>
		</parent>
	</root>`
	elem := parseOne(t, xml, "parent")

	// Find first element child (skip whitespace text nodes)
	var child *XMLElement
	for _, c := range elem.children {
		if e, ok := c.(*XMLElement); ok {
			child = e
			break
		}
	}
	if child == nil {
		t.Fatal("expected child")
	}
	if child.Name != "ns:child" {
		t.Errorf("expected 'ns:child', got %q", child.Name)
	}
}

func TestNamespaceOverride(t *testing.T) {
	xml := `<root xmlns:ns="http://outer.com">
		<parent xmlns:ns="http://inner.com">
			<ns:child>overridden</ns:child>
		</parent>
	</root>`
	elem := parseOne(t, xml, "parent")

	// Find first element child (skip whitespace text nodes)
	var child *XMLElement
	for _, c := range elem.children {
		if e, ok := c.(*XMLElement); ok {
			child = e
			break
		}
	}
	if child == nil {
		t.Fatal("expected child element")
	}
	if child.Name != "ns:child" {
		t.Errorf("expected 'ns:child', got %q", child.Name)
	}
}

// =============================================================================
// WHITESPACE TESTS
// =============================================================================

func TestWhitespacePreserved(t *testing.T) {
	xml := `<root><item>  spaced  </item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "  spaced  " {
		t.Errorf("expected '  spaced  ', got %q", elem.InnerText())
	}
}

func TestNewlinesInText(t *testing.T) {
	xml := "<root><item>line1\nline2\nline3</item></root>"
	elem := parseOne(t, xml, "item")

	expected := "line1\nline2\nline3"
	if elem.InnerText() != expected {
		t.Errorf("expected %q, got %q", expected, elem.InnerText())
	}
}

func TestTabsInText(t *testing.T) {
	xml := "<root><item>col1\tcol2\tcol3</item></root>"
	elem := parseOne(t, xml, "item")

	expected := "col1\tcol2\tcol3"
	if elem.InnerText() != expected {
		t.Errorf("expected %q, got %q", expected, elem.InnerText())
	}
}

func TestMixedWhitespace(t *testing.T) {
	xml := "<root><item> \t\n mixed \n\t </item></root>"
	elem := parseOne(t, xml, "item")

	expected := " \t\n mixed \n\t "
	if elem.InnerText() != expected {
		t.Errorf("expected %q, got %q", expected, elem.InnerText())
	}
}

// =============================================================================
// STREAMING BEHAVIOR TESTS
// =============================================================================

func TestStreamSpecificElements(t *testing.T) {
	xml := `<root>
		<item>1</item>
		<other>skip</other>
		<item>2</item>
		<other>skip</other>
		<item>3</item>
	</root>`
	elements := parseAll(t, xml, []string{"item"})

	if len(elements) != 3 {
		t.Errorf("expected 3 items, got %d", len(elements))
	}
}

func TestStreamMultipleNames(t *testing.T) {
	xml := `<root>
		<item>item1</item>
		<product>product1</product>
		<item>item2</item>
		<product>product2</product>
	</root>`
	elements := parseAll(t, xml, []string{"item", "product"})

	if len(elements) != 4 {
		t.Errorf("expected 4 elements, got %d", len(elements))
	}
}

func TestStreamNoMatch(t *testing.T) {
	xml := `<root><item>1</item><item>2</item></root>`
	elements := parseAll(t, xml, []string{"nonexistent"})

	if len(elements) != 0 {
		t.Errorf("expected 0 elements, got %d", len(elements))
	}
}

func TestStreamEmptyNames(t *testing.T) {
	xml := `<root><item>1</item><item>2</item></root>`
	elements := parseAll(t, xml, []string{})

	if len(elements) != 0 {
		t.Errorf("expected 0 elements with empty streamNames, got %d", len(elements))
	}
}

func TestStreamNilNames(t *testing.T) {
	xml := `<root><item>1</item><item>2</item></root>`
	elements := parseAll(t, xml, nil)

	if len(elements) != 0 {
		t.Errorf("expected 0 elements with nil streamNames, got %d", len(elements))
	}
}

// =============================================================================
// CONTEXT CANCELLATION TESTS
// =============================================================================

func TestContextCancellation(t *testing.T) {
	xml := `<root><item>1</item><item>2</item><item>3</item><item>4</item><item>5</item></root>`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, 1)

	count := 0
	for range parser.Stream() {
		count++
		if count >= 2 {
			cancel()
		}
	}

	// Should have stopped early due to cancellation
	t.Logf("Received %d elements before/after cancellation", count)
}

// =============================================================================
// XPATH NODE TYPE TESTS
// =============================================================================

func TestXPathTextNode(t *testing.T) {
	xml := `<root><item>hello world</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("text()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("text() expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Errorf("text() expected 1 text node, got %d", len(elements))
	}
}

func TestXPathTextNodeMultiple(t *testing.T) {
	// Mixed content: text, element, text
	xml := `<root><item>before<child/>after</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("text()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("text() expected []any, got %T", result)
	}
	// Should return 2 text nodes: "before" and "after"
	if len(elements) != 2 {
		t.Errorf("text() on mixed content expected 2 text nodes, got %d", len(elements))
	}
}

func TestXPathCommentNode(t *testing.T) {
	xml := `<root><item><!-- this is a comment -->text</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("comment()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("comment() expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Errorf("comment() expected 1 comment node, got %d", len(elements))
	}
}

func TestXPathNodeTypeElement(t *testing.T) {
	xml := `<root><parent><child>text</child></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("child")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}

	child, ok := elements[0].(*XMLElement)
	if !ok {
		t.Fatalf("expected *XMLElement, got %T", elements[0])
	}
	if child.Name != "child" {
		t.Errorf("expected 'child', got %q", child.Name)
	}
}

func TestXPathNodeTypeAttribute(t *testing.T) {
	xml := `<root><item id="123" name="test">content</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("@*")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("@* expected []any, got %T", result)
	}
	if len(elements) != 2 {
		t.Errorf("@* expected 2 attributes, got %d", len(elements))
	}

	for _, e := range elements {
		attr, ok := e.(*XMLAttribute)
		if !ok {
			t.Errorf("expected *XMLAttribute, got %T", e)
		}
		if attr.Name != "id" && attr.Name != "name" {
			t.Errorf("unexpected attribute name: %s", attr.Name)
		}
	}
}

func TestXPathNodeTypeRoot(t *testing.T) {
	xml := `<root><item>content</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("/")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("/ expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Errorf("/ expected 1 root node, got %d", len(elements))
	}
}

func TestXPathNodeFunction(t *testing.T) {
	xml := `<root><parent><child>text</child></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("node()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("node() expected []any, got %T", result)
	}
	// node() should return child element (and text nodes if supported)
	if len(elements) < 1 {
		t.Errorf("node() expected at least 1 node, got %d", len(elements))
	}
}

func TestXPathDescendantText(t *testing.T) {
	xml := `<root><parent><a>text1</a><b>text2</b></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("descendant::text()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("descendant::text() expected []any, got %T", result)
	}
	// Should find text nodes in <a> and <b>
	if len(elements) != 2 {
		t.Errorf("descendant::text() expected 2 text nodes, got %d", len(elements))
	}
}

// =============================================================================
// XPATH EVALUATION TESTS
// =============================================================================

func TestXPathSelectChild(t *testing.T) {
	xml := `<root><parent><child>found</child></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("child")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}

	child, ok := elements[0].(*XMLElement)
	if !ok {
		t.Fatalf("expected *XMLElement, got %T", elements[0])
	}
	if child.Name != "child" {
		t.Errorf("expected 'child', got %q", child.Name)
	}
}

func TestXPathSelectDescendant(t *testing.T) {
	xml := `<root><parent><a><b><target>deep</target></b></a></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("descendant::target")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}
}

func TestXPathSelectAttribute(t *testing.T) {
	xml := `<root><item id="123" name="test">content</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("@id")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	t.Logf("XPath @id result type: %T, value: %v", result, result)
}

func TestXPathCount(t *testing.T) {
	xml := `<root><parent><child>1</child><child>2</child><child>3</child></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("count(child)")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	count, ok := result.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T: %v", result, result)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %v", count)
	}
}

func TestXPathPredicate(t *testing.T) {
	xml := `<root><parent>
		<child type="a">first</child>
		<child type="b">second</child>
		<child type="a">third</child>
	</parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("child[@type='a']")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 2 {
		t.Errorf("expected 2 elements with type='a', got %d", len(elements))
	}
}

// =============================================================================
// MEMORY/POOL TESTS
// =============================================================================

func TestReleaseElement(t *testing.T) {
	xml := `<root><item>test</item></root>`
	elem := parseOne(t, xml, "item")

	// Should not panic
	elem.Release()
}

func TestReleaseWithChildren(t *testing.T) {
	xml := `<root><parent><a/><b/><c/></parent></root>`
	elem := parseOne(t, xml, "parent")

	// Should release all children too
	elem.Release()
}

// =============================================================================
// EDGE CASES AND SPECIAL XML
// =============================================================================

func TestXMLDeclaration(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><root><item>text</item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "text" {
		t.Errorf("expected 'text', got %q", elem.InnerText())
	}
}

func TestXMLWithDoctype(t *testing.T) {
	xml := `<?xml version="1.0"?><!DOCTYPE root><root><item>text</item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "text" {
		t.Errorf("expected 'text', got %q", elem.InnerText())
	}
}

func TestXMLComments(t *testing.T) {
	xml := `<root><!-- comment --><item>text</item><!-- another --></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "text" {
		t.Errorf("expected 'text', got %q", elem.InnerText())
	}
}

func TestProcessingInstruction(t *testing.T) {
	xml := `<?xml version="1.0"?><?custom instruction?><root><item>text</item></root>`
	elem := parseOne(t, xml, "item")

	if elem.InnerText() != "text" {
		t.Errorf("expected 'text', got %q", elem.InnerText())
	}
}

func TestMixedContent(t *testing.T) {
	xml := `<root><item>text<child/>more</item></root>`
	elem := parseOne(t, xml, "item")

	// Mixed content should preserve all text segments
	text := elem.InnerText()
	if text != "textmore" {
		t.Errorf("expected 'textmore', got %q", text)
	}
}

func TestMixedContentNonSelfClosing(t *testing.T) {
	xml := `<root><item>Hello <child>World</child> !</item></root>`
	elem := parseOne(t, xml, "item")

	// InnerText returns ALL descendant text content (standard DOM behavior)
	// "Hello " + "World" + " !" = "Hello World !"
	text := elem.InnerText()
	if text != "Hello World !" {
		t.Errorf("expected 'Hello World !', got %q", text)
	}

	// Find child element
	var child *XMLElement
	for _, c := range elem.children {
		if e, ok := c.(*XMLElement); ok {
			child = e
			break
		}
	}
	if child == nil {
		t.Fatal("expected child element")
	}
	if child.InnerText() != "World" {
		t.Errorf("expected child text 'World', got %q", child.InnerText())
	}
}

func TestUnicodeContent(t *testing.T) {
	xml := `<root><item>日本語 中文 한국어 emoji: 🎉</item></root>`
	elem := parseOne(t, xml, "item")

	expected := "日本語 中文 한국어 emoji: 🎉"
	if elem.InnerText() != expected {
		t.Errorf("expected %q, got %q", expected, elem.InnerText())
	}
}

func TestUnicodeInAttribute(t *testing.T) {
	xml := `<root><item name="日本語">text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(elem.Attributes))
	}
	if elem.Attributes[0].Value != "日本語" {
		t.Errorf("expected '日本語', got %q", elem.Attributes[0].Value)
	}
}

func TestVeryLongText(t *testing.T) {
	longText := strings.Repeat("a", 100000)
	xml := `<root><item>` + longText + `</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.InnerText()) != 100000 {
		t.Errorf("expected 100000 chars, got %d", len(elem.InnerText()))
	}
}

func TestManyAttributes(t *testing.T) {
	var attrs []string
	for i := 0; i < 100; i++ {
		attrs = append(attrs, `attr`+string(rune('0'+i/10))+string(rune('0'+i%10))+`="value"`)
	}
	xml := `<root><item ` + strings.Join(attrs, " ") + `>text</item></root>`
	elem := parseOne(t, xml, "item")

	if len(elem.Attributes) != 100 {
		t.Errorf("expected 100 attributes, got %d", len(elem.Attributes))
	}
}

func TestManyChildren(t *testing.T) {
	var xmlChildren []string
	for i := 0; i < 1000; i++ {
		xmlChildren = append(xmlChildren, `<child/>`)
	}
	xml := `<root><parent>` + strings.Join(xmlChildren, "") + `</parent></root>`
	elem := parseOne(t, xml, "parent")

	if len(elem.children) != 1000 {
		t.Errorf("expected 1000 children, got %d", len(elem.children))
	}
}

func TestEmptyDocument(t *testing.T) {
	xml := ``
	elements := parseAll(t, xml, []string{"item"})

	if len(elements) != 0 {
		t.Errorf("expected 0 elements from empty doc, got %d", len(elements))
	}
}

func TestRootOnly(t *testing.T) {
	xml := `<root/>`
	elem := parseOne(t, xml, "root")

	if elem.Name != "root" {
		t.Errorf("expected 'root', got %q", elem.Name)
	}
}

// =============================================================================
// BUFFER SIZE TESTS
// =============================================================================

func TestSmallBufferSize(t *testing.T) {
	xml := `<root><item>1</item><item>2</item><item>3</item></root>`
	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, 1)

	count := 0
	for range parser.Stream() {
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 elements, got %d", count)
	}
}

func TestZeroBufferSize(t *testing.T) {
	xml := `<root><item>1</item></root>`
	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, 0)

	count := 0
	for range parser.Stream() {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 element, got %d", count)
	}
}

func TestNegativeBufferSize(t *testing.T) {
	xml := `<root><item>1</item></root>`
	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, -10)

	count := 0
	for range parser.Stream() {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 element, got %d", count)
	}
}

// =============================================================================
// REAL-WORLD XML PATTERNS
// =============================================================================

func TestRSSFeed(t *testing.T) {
	xml := `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Example Feed</title>
    <item>
      <title>First Post</title>
      <link>http://example.com/1</link>
      <description>Description 1</description>
    </item>
    <item>
      <title>Second Post</title>
      <link>http://example.com/2</link>
      <description>Description 2</description>
    </item>
  </channel>
</rss>`

	elements := parseAll(t, xml, []string{"item"})
	if len(elements) != 2 {
		t.Errorf("expected 2 RSS items, got %d", len(elements))
	}
}

func TestAtomFeed(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example Feed</title>
  <entry>
    <title>Entry 1</title>
    <id>urn:uuid:1</id>
  </entry>
  <entry>
    <title>Entry 2</title>
    <id>urn:uuid:2</id>
  </entry>
</feed>`

	elements := parseAll(t, xml, []string{"entry"})
	if len(elements) != 2 {
		t.Errorf("expected 2 Atom entries, got %d", len(elements))
	}
}

func TestSoapEnvelope(t *testing.T) {
	xml := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Header/>
  <soap:Body>
    <m:GetPrice xmlns:m="http://example.com/prices">
      <m:Item>Apples</m:Item>
    </m:GetPrice>
  </soap:Body>
</soap:Envelope>`

	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"soap:Body"}, 10)

	count := 0
	for range parser.Stream() {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 SOAP body, got %d", count)
	}
}

func TestSVG(t *testing.T) {
	xml := `<?xml version="1.0"?>
<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
  <rect x="10" y="10" width="80" height="80" fill="red"/>
  <circle cx="50" cy="50" r="30" fill="blue"/>
</svg>`

	elements := parseAll(t, xml, []string{"rect", "circle"})
	if len(elements) != 2 {
		t.Errorf("expected 2 SVG shapes, got %d", len(elements))
	}
}

func TestXHTML(t *testing.T) {
	xml := `<?xml version="1.0"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Test</title></head>
  <body>
    <div id="content">
      <p>Paragraph 1</p>
      <p>Paragraph 2</p>
    </div>
  </body>
</html>`

	elements := parseAll(t, xml, []string{"p"})
	if len(elements) != 2 {
		t.Errorf("expected 2 paragraphs, got %d", len(elements))
	}
}

// =============================================================================
// BENCHMARKS
// =============================================================================

func BenchmarkParseSmall(b *testing.B) {
	xml := `<root><item>test</item></root>`
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, 10)
		for range parser.Stream() {
		}
	}
}

func BenchmarkParseMedium(b *testing.B) {
	var items []string
	for i := 0; i < 100; i++ {
		items = append(items, `<item id="`+string(rune(i))+`">content</item>`)
	}
	xml := `<root>` + strings.Join(items, "") + `</root>`
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, 100)
		for range parser.Stream() {
		}
	}
}

func BenchmarkParseLarge(b *testing.B) {
	var items []string
	for i := 0; i < 10000; i++ {
		items = append(items, `<item><name>Item</name><value>12345</value></item>`)
	}
	xml := `<root>` + strings.Join(items, "") + `</root>`
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, 1000)
		for range parser.Stream() {
		}
	}
}

func BenchmarkXPathQuery(b *testing.B) {
	xml := `<root><parent><child>1</child><child>2</child><child>3</child></parent></root>`
	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"parent"}, 10)

	var elem *XMLElement
	for e := range parser.Stream() {
		elem = e
	}

	expr, _ := xpath.Compile("child")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		elem.Evaluate(expr)
	}
}

// =============================================================================
// ELEMENTSTRING TESTS
// =============================================================================

func TestElementStringWithElement(t *testing.T) {
	xml := `<root><item>hello</item></root>`
	elem := parseOne(t, xml, "item")
	result := ElementString(elem.Evaluate(xpath.MustCompile(".")))
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestElementStringWithAttribute(t *testing.T) {
	xml := `<root><item id="42">text</item></root>`
	elem := parseOne(t, xml, "item")
	result := ElementString(elem.Evaluate(xpath.MustCompile("@id")))
	if result != "42" {
		t.Errorf("expected '42', got %q", result)
	}
}

func TestElementStringWithString(t *testing.T) {
	result := ElementString("direct string")
	if result != "direct string" {
		t.Errorf("expected 'direct string', got %q", result)
	}
}

func TestElementStringWithEmptySlice(t *testing.T) {
	result := ElementString([]any{})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestElementStringWithUnknownType(t *testing.T) {
	result := ElementString(12345)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// =============================================================================
// ELEMENTSTRING WITH CONTENT NODE TESTS
// =============================================================================

func TestElementStringWithContentNode(t *testing.T) {
	xml := `<root><item>hello world</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("text()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	str := ElementString(result)
	if str != "hello world" {
		t.Errorf("expected 'hello world', got %q", str)
	}
}

func TestElementStringWithContentNodeMultiple(t *testing.T) {
	xml := `<root><item>before<child/>after</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("text()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	// ElementString returns InnerText of the first text node
	result := elem.Evaluate(expr)
	str := ElementString(result)
	if str != "before" {
		t.Errorf("expected 'before', got %q", str)
	}
}

// =============================================================================
// XPATH STRING FUNCTION TESTS
// =============================================================================

func TestXPathStringFunction(t *testing.T) {
	xml := `<root><item>hello</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("string()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T: %v", result, result)
	}
	if str != "hello" {
		t.Errorf("expected 'hello', got %q", str)
	}
}

func TestXPathConcatFunction(t *testing.T) {
	xml := `<root><item><a>hello</a><b>world</b></item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("concat(a, ' ', b)")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T: %v", result, result)
	}
	if str != "hello world" {
		t.Errorf("expected 'hello world', got %q", str)
	}
}

func TestElementStringWithXPathStringResult(t *testing.T) {
	xml := `<root><item>hello</item></root>`
	elem := parseOne(t, xml, "item")

	expr, err := xpath.Compile("string()")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	str := ElementString(elem.Evaluate(expr))
	if str != "hello" {
		t.Errorf("expected 'hello', got %q", str)
	}
}

// =============================================================================
// XPATH SIBLING NAVIGATION TESTS
// =============================================================================

func TestXPathFollowingSibling(t *testing.T) {
	xml := `<root><parent><a>1</a><b>2</b><c>3</c></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("a/following-sibling::*")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 2 {
		t.Fatalf("expected 2 following siblings, got %d", len(elements))
	}

	names := make([]string, len(elements))
	for i, e := range elements {
		names[i] = e.(*XMLElement).Name
	}
	if names[0] != "b" || names[1] != "c" {
		t.Errorf("expected [b, c], got %v", names)
	}
}

func TestXPathPrecedingSibling(t *testing.T) {
	xml := `<root><parent><a>1</a><b>2</b><c>3</c></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("c/preceding-sibling::*")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 2 {
		t.Fatalf("expected 2 preceding siblings, got %d", len(elements))
	}
}

func TestXPathFollowingSiblingByName(t *testing.T) {
	xml := `<root><parent><a/><b/><a/><c/></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("b/following-sibling::a")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Errorf("expected 1 following sibling named 'a', got %d", len(elements))
	}
}

// =============================================================================
// XPATH PARENT AXIS TESTS
// =============================================================================

func TestXPathParentAxis(t *testing.T) {
	xml := `<root><parent><child>text</child></parent></root>`
	elem := parseOne(t, xml, "parent")

	expr, err := xpath.Compile("child/..")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 parent, got %d", len(elements))
	}

	parent, ok := elements[0].(*XMLElement)
	if !ok {
		t.Fatalf("expected *XMLElement, got %T", elements[0])
	}
	if parent.Name != "parent" {
		t.Errorf("expected 'parent', got %q", parent.Name)
	}
}

func TestXPathParentAxisExplicit(t *testing.T) {
	// parent::* matches only ElementNode, not RootNode.
	// The streamed element is the subtree root, so child/parent::* won't match it.
	// Use a deeper structure where the parent is not the root.
	xml := `<root><outer><inner><target>text</target></inner></outer></root>`
	elem := parseOne(t, xml, "outer")

	expr, err := xpath.Compile("inner/target/parent::*")
	if err != nil {
		t.Fatalf("failed to compile xpath: %v", err)
	}

	result := elem.Evaluate(expr)
	elements, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 parent, got %d", len(elements))
	}
	if elements[0].(*XMLElement).Name != "inner" {
		t.Errorf("expected 'inner', got %q", elements[0].(*XMLElement).Name)
	}
}

// =============================================================================
// MULTIPLE STREAM CALLS TEST
// =============================================================================

func TestMultipleStreamCalls(t *testing.T) {
	xml := `<root><item>1</item><item>2</item><item>3</item></root>`
	ctx := context.Background()
	parser := NewParser(ctx, strings.NewReader(xml), []string{"item"}, 10)

	ch1 := parser.Stream()
	ch2 := parser.Stream()

	// Must return the same channel
	if ch1 != ch2 {
		t.Error("expected Stream() to return the same channel on subsequent calls")
	}

	// Should still work correctly
	count := 0
	for range ch1 {
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 elements, got %d", count)
	}
}

// =============================================================================
// CONCURRENT RELEASE TESTS
// =============================================================================

func TestConcurrentRelease(t *testing.T) {
	xml := `<root>`
	for i := 0; i < 100; i++ {
		xml += `<item><child>text</child></item>`
	}
	xml += `</root>`

	elements := parseAll(t, xml, []string{"item"})
	if len(elements) != 100 {
		t.Fatalf("expected 100 elements, got %d", len(elements))
	}

	// Release all elements concurrently
	var wg sync.WaitGroup
	wg.Add(len(elements))
	for _, elem := range elements {
		go func(e *XMLElement) {
			defer wg.Done()
			e.Release()
		}(elem)
	}
	wg.Wait()
}

// =============================================================================
// ERROR READER TESTS
// =============================================================================

type errorReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errorReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	// Return a chunk then error on next read
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestErrorReaderMidStream(t *testing.T) {
	// Partial XML that cuts off mid-stream
	partial := `<root><item>1</item><item>2</item><item`
	reader := &errorReader{
		data: []byte(partial),
		err:  io.ErrUnexpectedEOF,
	}

	ctx := context.Background()
	parser := NewParser(ctx, reader, []string{"item"}, 10)

	count := 0
	for range parser.Stream() {
		count++
	}
	// Should get the 2 complete items before the error
	if count != 2 {
		t.Errorf("expected 2 complete elements before error, got %d", count)
	}
}

func TestErrorReaderEmpty(t *testing.T) {
	reader := &errorReader{
		data: nil,
		err:  io.ErrUnexpectedEOF,
	}

	ctx := context.Background()
	parser := NewParser(ctx, reader, []string{"item"}, 10)

	count := 0
	for range parser.Stream() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 elements from error reader, got %d", count)
	}
}

func BenchmarkElementRelease(b *testing.B) {
	xml := `<root><parent><a/><b/><c/></parent></root>`
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser(ctx, strings.NewReader(xml), []string{"parent"}, 10)
		for elem := range parser.Stream() {
			elem.Release()
		}
	}
}
