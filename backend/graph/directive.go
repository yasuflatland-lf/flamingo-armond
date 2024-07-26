package graph

import (
	"context"
	"errors"
	"regexp"

	"github.com/99designs/gqlgen/graphql"
)

//gqlgen

//
// #directive @constraint(
//#  minLength: Int,
//#  maxLength: Int,
//#  min: Int,
//#  max: Int,
//#  pattern: String) on INPUT_FIELD_DEFINITION

func Constraint(ctx context.Context, obj interface{}, next graphql.Resolver, minLength *int, maxLength *int, min *int, max *int, pattern *string) (interface{}, error) {
	val, err := next(ctx)
	if err != nil {
		return nil, err
	}

	switch v := val.(type) {
	case string:
		if minLength != nil && len(v) < *minLength {
			return nil, errors.New("value is too short")
		}
		if maxLength != nil && len(v) > *maxLength {
			return nil, errors.New("value is too long")
		}
		if pattern != nil {
			matched, _ := regexp.MatchString(*pattern, v)
			if !matched {
				return nil, errors.New("value does not match pattern")
			}
		}
	case int:
		if min != nil && v < *min {
			return nil, errors.New("value is too small")
		}
		if max != nil && v > *max {
			return nil, errors.New("value is too large")
		}
	}

	return val, nil
}
