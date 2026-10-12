package gqlH

import (
	"errors"
	"strconv"

	graphql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

const (
	maxGraphQLBodyBytes      = 32 << 10
	maxGraphQLQueryBytes     = 16 << 10
	maxGraphQLVariablesBytes = 16 << 10
	maxGraphQLDepth          = 6
	maxGraphQLFields         = 100
	maxGraphQLComplexity     = 1000
	defaultListSize          = 20
)

type graphqlCost struct {
	fields     int
	complexity int
	depth      int
}

// Count every selected field, including aliases and each fragment expansion.
// A list multiplies its descendants by its requested page size or a conservative
// default. Costs saturate above the limit so multiplication cannot overflow.
func estimateGraphQLCost(doc *ast.Document, schema graphql.Schema, variables map[string]any) (graphqlCost, error) {
	fragments := make(map[string]*ast.FragmentDefinition)
	for _, definition := range doc.Definitions {
		if fragment, ok := definition.(*ast.FragmentDefinition); ok {
			fragments[fragment.Name.Value] = fragment
		}
	}
	var total graphqlCost
	operations := 0
	for _, definition := range doc.Definitions {
		operation, ok := definition.(*ast.OperationDefinition)
		if !ok {
			continue
		}
		operations++
		var root *graphql.Object
		switch operation.Operation {
		case ast.OperationTypeQuery:
			root = schema.QueryType()
		case ast.OperationTypeMutation:
			root = schema.MutationType()
		default:
			return total, errors.New("unsupported operation")
		}
		cost, err := countSelections(operation.SelectionSet, root, 1, 1, 0, fragments, map[string]bool{}, variables)
		if err != nil {
			return total, err
		}
		total.fields += cost.fields
		total.complexity += cost.complexity
		if cost.depth > total.depth {
			total.depth = cost.depth
		}
		if total.fields > maxGraphQLFields || total.complexity > maxGraphQLComplexity || total.depth > maxGraphQLDepth {
			return total, nil
		}
	}
	if operations == 0 {
		return total, errors.New("missing operation")
	}
	return total, nil
}

func countSelections(set *ast.SelectionSet, parent *graphql.Object, depth, multiplier, pageSize int, fragments map[string]*ast.FragmentDefinition, visiting map[string]bool, variables map[string]any) (graphqlCost, error) {
	var cost graphqlCost
	if set == nil {
		return cost, nil
	}
	for _, selection := range set.Selections {
		var child graphqlCost
		var err error
		switch node := selection.(type) {
		case *ast.Field:
			cost.fields++
			if depth > cost.depth {
				cost.depth = depth
			}
			var childType *graphql.Object
			childMultiplier := multiplier
			childPageSize := pageSize
			if parent != nil {
				if definition := parent.Fields()[node.Name.Value]; definition != nil {
					childType, _ = graphql.GetNamed(definition.Type).(*graphql.Object)
					if isListType(definition.Type) {
						childMultiplier *= listSize(node.Name.Value, node.Arguments, variables, pageSize)
						childPageSize = 0
					} else if childType != nil {
						// Paginated wrappers carry the page size on the parent
						// object, while their items field is the actual list.
						if requested := requestedListSize(node.Arguments, variables); requested > childPageSize {
							childPageSize = requested
						}
					}
				}
			}
			if childMultiplier > maxGraphQLComplexity+1 {
				childMultiplier = maxGraphQLComplexity + 1
			}
			cost.complexity += childMultiplier
			if cost.fields > maxGraphQLFields || cost.complexity > maxGraphQLComplexity || depth > maxGraphQLDepth {
				return cost, nil
			}
			child, err = countSelections(node.SelectionSet, childType, depth+1, childMultiplier, childPageSize, fragments, visiting, variables)
		case *ast.InlineFragment:
			child, err = countSelections(node.SelectionSet, parent, depth, multiplier, pageSize, fragments, visiting, variables)
		case *ast.FragmentSpread:
			name := node.Name.Value
			fragment := fragments[name]
			if fragment == nil || visiting[name] {
				return cost, errors.New("unknown or cyclic fragment")
			}
			visiting[name] = true
			child, err = countSelections(fragment.SelectionSet, parent, depth, multiplier, pageSize, fragments, visiting, variables)
			delete(visiting, name)
		}
		if err != nil {
			return cost, err
		}
		cost.fields += child.fields
		cost.complexity += child.complexity
		if child.depth > cost.depth {
			cost.depth = child.depth
		}
		if cost.fields > maxGraphQLFields || cost.complexity > maxGraphQLComplexity || cost.depth > maxGraphQLDepth {
			return cost, nil
		}
	}
	return cost, nil
}

func isListType(t graphql.Type) bool {
	for {
		switch typed := t.(type) {
		case *graphql.NonNull:
			t = typed.OfType
		case *graphql.List:
			return true
		default:
			return false
		}
	}
}

func listSize(fieldName string, args []*ast.Argument, variables map[string]any, inherited int) int {
	size := defaultListSize
	if inherited > size {
		size = inherited
	}
	if requested := requestedListSize(args, variables); requested > size {
		size = requested
	}
	// Task image lists are validated and persisted with a hard maximum of five
	// entries. Charging the generic default of twenty for each nested list would
	// reject the normal task page without reflecting the actual fan-out.
	if (fieldName == "image_keys" || fieldName == "image_urls") && size > 5 {
		size = 5
	}
	return size
}

func requestedListSize(args []*ast.Argument, variables map[string]any) int {
	size := 0
	for _, arg := range args {
		if arg.Name.Value != "limit" && arg.Name.Value != "page_size" && arg.Name.Value != "first" && arg.Name.Value != "last" {
			continue
		}
		var value int
		switch typed := arg.Value.(type) {
		case *ast.IntValue:
			value, _ = strconv.Atoi(typed.Value)
		case *ast.Variable:
			if n, ok := variables[typed.Name.Value].(float64); ok && n > 0 {
				if n > float64(maxGraphQLComplexity) {
					value = maxGraphQLComplexity + 1
				} else {
					value = int(n)
				}
			}
		}
		if value > size {
			size = value
		}
	}
	if size > maxGraphQLComplexity+1 {
		return maxGraphQLComplexity + 1
	}
	return size
}
