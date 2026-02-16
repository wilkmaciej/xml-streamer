package xmlstreamer

import (
	"strings"

	"github.com/wilkmaciej/xpath"
)

type elementNavigator struct {
	root *XMLElement
	// Current node can be *XMLElement or *XMLContentNode
	currNode XMLNode
	// Cached reference to current element (for attribute access)
	currElement *XMLElement
	// Index of the current attribute, -1 if not on an attribute
	attributeIndex int
}

func (navigator *elementNavigator) NodeType() xpath.NodeType {
	if navigator.attributeIndex != -1 {
		return xpath.AttributeNode
	}
	switch node := navigator.currNode.(type) {
	case *XMLContentNode:
		return node.nodeType
	case *XMLElement:
		if node == navigator.root && node.parent == nil {
			return xpath.RootNode
		}
		return xpath.ElementNode
	}
	return xpath.ElementNode
}

// LocalName returns the local name of the current node
func (navigator *elementNavigator) LocalName() string {
	if navigator.attributeIndex != -1 {
		name := navigator.currElement.Attributes[navigator.attributeIndex].Name
		if idx := strings.IndexByte(name, ':'); idx != -1 {
			return name[idx+1:]
		}
		return name
	}
	if navigator.currElement != nil {
		return navigator.currElement.localName
	}
	// Text and comment nodes have no local name
	return ""
}

// Prefix returns the namespace prefix of the current node
func (navigator *elementNavigator) Prefix() string {
	if navigator.attributeIndex != -1 {
		name := navigator.currElement.Attributes[navigator.attributeIndex].Name
		if idx := strings.IndexByte(name, ':'); idx != -1 {
			return name[:idx]
		}
		return ""
	}
	if navigator.currElement != nil {
		return navigator.currElement.prefix
	}
	// Text and comment nodes have no prefix
	return ""
}

// NamespaceURL returns the namespace URI of the current node
// URL should be URI but kept for compatibility
func (navigator *elementNavigator) NamespaceURL() string {
	if navigator.attributeIndex != -1 {
		// For attributes, check if they have a namespace prefix
		attrName := navigator.currElement.Attributes[navigator.attributeIndex].Name
		if idx := strings.IndexByte(attrName, ':'); idx != -1 {
			prefix := attrName[:idx]
			if navigator.currElement.namespaces != nil {
				return navigator.currElement.namespaces[prefix]
			}
		}
		return ""
	}
	if navigator.currElement != nil {
		return navigator.currElement.namespaceURI
	}
	// Text and comment nodes inherit namespace from parent
	return ""
}

// Value returns the string value of the current node
func (navigator *elementNavigator) Value() string {
	if navigator.attributeIndex != -1 {
		return navigator.currElement.Attributes[navigator.attributeIndex].Value
	}
	return navigator.currNode.InnerText()
}

func (navigator *elementNavigator) Copy() xpath.NodeNavigator {
	navCopy := *navigator
	return &navCopy
}

func (navigator *elementNavigator) MoveToRoot() {
	navigator.currNode = navigator.root
	navigator.currElement = navigator.root
	navigator.attributeIndex = -1
}

func (navigator *elementNavigator) MoveToParent() bool {
	if navigator.attributeIndex != -1 {
		navigator.attributeIndex = -1
		return true
	}
	parent := navigator.currNode.Parent()
	if parent != nil {
		navigator.currNode = parent
		navigator.currElement = parent
		navigator.attributeIndex = -1
		return true
	}
	return false
}

func (navigator *elementNavigator) MoveToNextAttribute() bool {
	if navigator.currElement == nil {
		return false
	}
	if navigator.attributeIndex >= len(navigator.currElement.Attributes)-1 {
		return false
	}
	navigator.attributeIndex++
	return true
}

// MoveToChild moves to the first child node
func (navigator *elementNavigator) MoveToChild() bool {
	if navigator.attributeIndex != -1 {
		return false
	}
	if navigator.currElement == nil {
		return false
	}
	if len(navigator.currElement.children) > 0 {
		child := navigator.currElement.children[0]
		navigator.currNode = child
		if elem, ok := child.(*XMLElement); ok {
			navigator.currElement = elem
		} else {
			navigator.currElement = nil
		}
		return true
	}
	return false
}

// MoveToFirst moves to the first sibling node
func (navigator *elementNavigator) MoveToFirst() bool {
	if navigator.attributeIndex != -1 {
		return false
	}
	parent := navigator.currNode.Parent()
	if parent == nil {
		return false
	}
	// Check if we're already first using O(1) index check
	if navigator.currNode.getSiblingIndex() == 0 {
		return false
	}
	// Move to first child
	if len(parent.children) > 0 {
		first := parent.children[0]
		navigator.currNode = first
		if elem, ok := first.(*XMLElement); ok {
			navigator.currElement = elem
		} else {
			navigator.currElement = nil
		}
		return true
	}
	return false
}

// MoveToNext moves to the next sibling node
func (navigator *elementNavigator) MoveToNext() bool {
	if navigator.attributeIndex != -1 {
		return false
	}
	parent := navigator.currNode.Parent()
	if parent == nil {
		return false
	}
	// Use O(1) sibling index lookup instead of O(n) linear search
	idx := navigator.currNode.getSiblingIndex()
	if idx+1 >= len(parent.children) {
		return false
	}
	next := parent.children[idx+1]
	navigator.currNode = next
	if elem, ok := next.(*XMLElement); ok {
		navigator.currElement = elem
	} else {
		navigator.currElement = nil
	}
	return true
}

// MoveToPrevious moves to the previous sibling node
func (navigator *elementNavigator) MoveToPrevious() bool {
	if navigator.attributeIndex != -1 {
		return false
	}
	parent := navigator.currNode.Parent()
	if parent == nil {
		return false
	}
	// Use O(1) sibling index lookup instead of O(n) linear search
	idx := navigator.currNode.getSiblingIndex()
	if idx <= 0 {
		return false
	}
	prev := parent.children[idx-1]
	navigator.currNode = prev
	if elem, ok := prev.(*XMLElement); ok {
		navigator.currElement = elem
	} else {
		navigator.currElement = nil
	}
	return true
}

// MoveTo moves this navigator to the same position as the specified navigator
func (navigator *elementNavigator) MoveTo(other xpath.NodeNavigator) bool {
	if otherNav, ok := other.(*elementNavigator); ok {
		if otherNav.root == navigator.root {
			navigator.currNode = otherNav.currNode
			navigator.currElement = otherNav.currElement
			navigator.attributeIndex = otherNav.attributeIndex
			return true
		}
	}
	return false
}

// String returns the string representation of the current node
func (navigator *elementNavigator) String() string {
	return navigator.Value()
}
