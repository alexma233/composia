package store

import "strings"

func appendStringFilterInClause(whereClause string, args []any, expression string, values []string) (string, []any) {
	return appendStringFilterClause(whereClause, args, expression, values, false)
}

func appendStringFilterNotInClause(whereClause string, args []any, expression string, values []string) (string, []any) {
	return appendStringFilterClause(whereClause, args, expression, values, true)
}

func appendStringFilterClause(whereClause string, args []any, expression string, values []string, exclude bool) (string, []any) {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		filtered = append(filtered, value)
	}
	if len(filtered) == 0 {
		return whereClause, args
	}

	whereClause += ` AND ` + expression
	if exclude {
		whereClause += ` NOT`
	}
	whereClause += ` IN (`
	var whereClauseSb28 strings.Builder
	for i, value := range filtered {
		if i > 0 {
			whereClauseSb28.WriteString(", ")
		}
		whereClauseSb28.WriteString("?")
		args = append(args, value)
	}
	whereClause += whereClauseSb28.String()
	whereClause += `)`

	return whereClause, args
}
