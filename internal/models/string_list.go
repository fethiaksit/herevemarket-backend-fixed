package models

import (
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// StringList ensures category fields can be decoded whether stored as a single
// string or an array of strings.
type StringList []string

// UnmarshalBSONValue accepts both string and array BSON types, allowing legacy
// documents to be decoded without failing the entire request.
func (s *StringList) UnmarshalBSONValue(t bsontype.Type, data []byte) error {
	switch t {
	case bsontype.Null:
		*s = nil
		return nil
	case bsontype.Array:
		var values []string
		if err := bson.UnmarshalValue(t, data, &values); err != nil {
			return err
		}
		*s = values
		return nil
	case bsontype.String:
		var value string
		if err := bson.UnmarshalValue(t, data, &value); err != nil {
			return err
		}

		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			*s = []string{}
			return nil
		}

		*s = []string{trimmed}
		return nil
	default:
		return fmt.Errorf("cannot decode %s into StringList", t)
	}
}

// MarshalBSONValue always stores the list as an array, keeping new writes
// consistent even when legacy documents used a string value.
func (s StringList) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue([]string(s))
}
