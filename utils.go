package xmlstreamer

// ElementString extracts a string from an XPath Evaluate result.
// For node-set results, it returns the InnerText of the first node.
// For string results, it returns the string directly.
// Returns "" for empty or unrecognized results.
func ElementString(input any) string {
	switch v := input.(type) {
	case []any:
		if len(v) == 0 {
			return ""
		}
		switch elem := v[0].(type) {
		case *XMLElement:
			return elem.InnerText()
		case *XMLContentNode:
			return elem.InnerText()
		case *XMLAttribute:
			return elem.Value
		default:
			return ""
		}
	case string:
		return v
	default:
		return ""
	}
}
