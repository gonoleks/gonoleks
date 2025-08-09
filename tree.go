package gonoleks

import (
	"strings"
)

// nodeType defines the classification of nodes in the routing tree
type nodeType uint8

const (
	static   nodeType = iota // Static path
	root                     // Root node
	param                    // Parameter (:id)
	catchAll                 // Wildcard (*)
)

// node represents a single element in the routing tree structure
type node struct {
	path     string           // Path segment this node represents
	param    *node            // Child parameter node (if any)
	children map[string]*node // Static child nodes mapped by path segment
	nType    nodeType         // Type classification of this node
	handlers handlersChain    // Handler functions associated with this node
}

// addRoute adds a node with the provided handlers to the specified path
// It parses the path into segments and builds the routing tree accordingly
func (n *node) addRoute(path string, handlers handlersChain) {
	currentNode := n
	originalPath := path
	path = path[1:] // Remove leading slash

	paramNames := make(map[string]bool)

	// Validate catch-all routes are only at the end
	if strings.Contains(originalPath, "*") {
		catchAllIndex := strings.Index(originalPath, "*")
		if catchAllIndex != -1 {
			// Find the next slash after the catch-all
			nextSlash := strings.Index(originalPath[catchAllIndex:], "/")
			if nextSlash != -1 {
				panic("catch-all routes are only allowed at the end of the path")
			}
		}
	}

	for {
		pathLen := len(path)
		if pathLen == 0 {
			n.setHandlers(currentNode, handlers)
			break
		}

		segmentDelimiter := strings.Index(path, "/")
		if segmentDelimiter == -1 {
			segmentDelimiter = pathLen
		}

		pathSegment := path[:segmentDelimiter]

		// Check for empty path segment
		if len(pathSegment) == 0 {
			// Skip empty segments (consecutive slashes)
			path = path[segmentDelimiter:]
			if len(path) > 0 {
				path = path[1:] // Skip the slash
			}
			continue
		}

		// Check for compound parameters like :from-:to
		if strings.Contains(pathSegment, ".:") || strings.Contains(pathSegment, "-:") {
			currentNode = n.handleCompoundSegment(currentNode, pathSegment, paramNames)
		} else if pathSegment[0] == ':' || pathSegment[0] == '*' {
			currentNode = n.handleParameterSegment(currentNode, pathSegment, originalPath, paramNames)
		} else {
			currentNode = n.handleStaticSegment(currentNode, pathSegment)
		}

		// Traverse to the next segment
		path = path[segmentDelimiter:]
		if len(path) > 0 {
			path = path[1:] // Skip the slash
		}
	}
}

// setHandlers assigns handler functions to a node, ensuring no duplicate routes exist
// It creates a deep copy of the handlers to prevent unintended modifications
func (n *node) setHandlers(currentNode *node, handlers handlersChain) {
	if currentNode.handlers != nil {
		return
	}

	// Make a deep copy of handler's references
	routeHandlers := make(handlersChain, len(handlers))
	copy(routeHandlers, handlers)

	currentNode.handlers = routeHandlers
}

// handleParameterSegment processes path segments that represent parameters (:param) or catch-all (*wildcard)
// It validates parameter conflicts and creates appropriate nodes in the routing tree
func (n *node) handleParameterSegment(currentNode *node, pathSegment, originalPath string, paramNames map[string]bool) *node {
	if currentNode.param != nil {
		if currentNode.param.path[0] == '*' {
			panic("parameter " + pathSegment + " conflicts with catch all (*) route in path '" + originalPath + "'")
		} else if currentNode.param.path != pathSegment {
			panic("parameter " + pathSegment + " in new path '" + originalPath + "' conflicts with existing wildcard '" + currentNode.param.path + "'")
		}
	}

	if currentNode.param == nil {
		var nType nodeType
		if pathSegment[0] == '*' {
			nType = catchAll
		} else {
			nType = param
		}

		currentNode.param = &node{
			path:     pathSegment,
			children: make(map[string]*node),
			nType:    nType,
		}
	}
	if pathSegment[0] == ':' {
		paramNames[pathSegment[1:]] = true
	}
	return currentNode.param
}

// handleStaticSegment processes literal path segments (non-parameter parts)
// It allows static routes to coexist with parameter routes, with static routes taking precedence during matching
func (n *node) handleStaticSegment(currentNode *node, pathSegment string) *node {
	childNode := currentNode.children[pathSegment]
	if childNode == nil {
		childNode = &node{
			path:     pathSegment,
			children: make(map[string]*node),
			nType:    static,
		}
		currentNode.children[pathSegment] = childNode
	}
	return childNode
}

// handleCompoundSegment processes path segments containing multiple parameters separated by delimiters
// It handles complex patterns like ":file.:ext" or ":from-:to" by creating specialized nodes
func (n *node) handleCompoundSegment(currentNode *node, pathSegment string, paramNames map[string]bool) *node {
	// Create a special node for compound segments
	childNode := currentNode.children[pathSegment]
	if childNode == nil {
		childNode = &node{
			path:     pathSegment,
			children: make(map[string]*node),
			nType:    static,
		}
		currentNode.children[pathSegment] = childNode
	}

	// Extract parameter names from the compound segment
	extractParamNames(pathSegment, paramNames)

	return childNode
}

// extractParamNames parses a compound path segment to identify and register parameter names
// It handles patterns like ":file.:ext" or ":from-:to" by detecting delimiter positions
func extractParamNames(pathSegment string, paramNames map[string]bool) {
	// Find all parameter parts in the segment
	parts := strings.Split(pathSegment, ":")

	// Skip the first part as it's before any parameter
	for i := 1; i < len(parts); i++ {
		part := parts[i]

		// Find where the parameter name ends (at . or -)
		end := len(part)
		dotIndex := strings.Index(part, ".")
		dashIndex := strings.Index(part, "-")

		if dotIndex != -1 && (dashIndex == -1 || dotIndex < dashIndex) {
			end = dotIndex
		} else if dashIndex != -1 {
			end = dashIndex
		}

		// Register the parameter name
		paramName := part[:end]
		paramNames[paramName] = true
	}
}

// matchRoute traverses the routing tree to find a matching route for the given path
// It populates the context with parameter values and returns the associated handlers if found
//
//go:noinline
func (n *node) matchRoute(path string, ctx *Context) handlersChain {
	currentNode := n
	// Optimized path preprocessing - avoid repeated slice operations
	pathStart := 0
	if len(path) > 0 && path[0] == '/' {
		pathStart = 1 // Skip leading slash without creating new slice
	}

	for {
		pathLen := len(path)
		if pathStart >= pathLen {
			// If we've reached the end of the path, check if current node has handlers
			if currentNode.handlers != nil {
				return currentNode.handlers
			}
			return nil
		}

		// Fast path: use strings.IndexByte for optimized slash finding
		segmentDelimiter := strings.IndexByte(path[pathStart:], '/')
		var segmentEnd int
		if segmentDelimiter == -1 {
			segmentEnd = pathLen
		} else {
			segmentEnd = pathStart + segmentDelimiter
		}

		// Check for empty path segment
		if pathStart == segmentEnd {
			// Skip empty segments (consecutive slashes)
			pathStart = segmentEnd + 1
			continue
		}

		pathSegment := path[pathStart:segmentEnd]

		// Try to match static route first
		if nextNode := currentNode.children[pathSegment]; nextNode != nil {
			currentNode = nextNode
		} else {
			// Check for compound parameter patterns
			matched := false
			// Pre-check if segment contains common delimiters to avoid unnecessary iterations
			hasDelimiters := strings.IndexByte(pathSegment, '.') != -1 || strings.IndexByte(pathSegment, '-') != -1
			if hasDelimiters {
				for pattern, node := range currentNode.children {
					// Quick pattern check using IndexByte instead of Contains
					if (strings.IndexByte(pattern, '.') != -1 && strings.IndexByte(pattern, ':') != -1) ||
						(strings.IndexByte(pattern, '-') != -1 && strings.IndexByte(pattern, ':') != -1) {
						if matchCompoundPattern(pattern, pathSegment, ctx) {
							currentNode = node
							matched = true
							break
						}
					}
				}
			}

			// If no compound match, try regular parameter match
			if !matched && currentNode.param != nil {
				switch currentNode.param.nType {
				case param:
					// Parameter match
					ctx.paramValues[currentNode.param.path[1:]] = pathSegment
					currentNode = currentNode.param
				case catchAll:
					// Catch-all match - capture the rest of the path
					paramName := "*"
					if len(currentNode.param.path) > 1 {
						paramName = currentNode.param.path[1:]
					}

					// For catch-all, capture remaining path without creating intermediate slices
					if segmentEnd < pathLen {
						ctx.paramValues[paramName] = path[pathStart:]
					} else {
						ctx.paramValues[paramName] = pathSegment
					}
					return currentNode.param.handlers
				default:
					return nil
				}
			} else if !matched {
				// No match found
				return nil
			}
		}

		// Traverse to the next segment - optimized without slice creation
		pathStart = segmentEnd
		if pathStart < pathLen && path[pathStart] == '/' {
			pathStart++ // Skip the slash
		}
	}
}

// matchCompoundPattern evaluates if a path segment matches a compound parameter pattern
// It handles patterns like ":file.:ext" or ":from-:to" and extracts parameter values
func matchCompoundPattern(pattern, segment string, ctx *Context) bool {
	// Quick check for required delimiters in segment
	hasDot := strings.IndexByte(segment, '.') != -1
	hasDash := strings.IndexByte(segment, '-') != -1

	// Early return if pattern contains a delimiter that segment doesn't have
	if (strings.Contains(pattern, ".:") && !hasDot) ||
		(strings.Contains(pattern, "-:") && !hasDash) {
		return false
	}

	// Handle complex patterns with multiple delimiters by parsing sequentially
	patternPos := 0
	segmentPos := 0

	for patternPos < len(pattern) {
		if pattern[patternPos] == ':' {
			// Find the end of the parameter name
			paramStart := patternPos + 1
			paramEnd := paramStart
			for paramEnd < len(pattern) && pattern[paramEnd] != '.' && pattern[paramEnd] != '-' {
				paramEnd++
			}
			paramName := pattern[paramStart:paramEnd]

			// Find the corresponding value in the segment
			valueStart := segmentPos
			valueEnd := segmentPos

			// If this is the last parameter, take the rest of the segment
			if paramEnd == len(pattern) {
				valueEnd = len(segment)
			} else {
				// Find the next delimiter in the segment
				delimiter := pattern[paramEnd]
				for valueEnd < len(segment) && segment[valueEnd] != delimiter {
					valueEnd++
				}
				if valueEnd == len(segment) {
					return false // Delimiter not found in segment
				}
			}

			// Extract the parameter value
			if valueEnd <= valueStart {
				return false // Empty parameter value
			}
			ctx.paramValues[paramName] = segment[valueStart:valueEnd]

			// Move positions forward
			patternPos = paramEnd
			segmentPos = valueEnd
		} else {
			// Handle literal characters (delimiters)
			if segmentPos >= len(segment) || pattern[patternPos] != segment[segmentPos] {
				return false
			}
			patternPos++
			segmentPos++
		}
	}

	// Ensure we've consumed the entire segment
	return segmentPos == len(segment)
}

// createRootNode initializes a new root node for the routing tree
// This serves as the entry point for all route matching operations
func createRootNode() *node {
	return &node{
		path:     "/",
		nType:    root,
		children: make(map[string]*node),
		handlers: nil,
	}
}
