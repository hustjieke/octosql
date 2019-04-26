package execution

import (
	"github.com/cube2222/octosql"
	"github.com/pkg/errors"
)

type Map struct {
	expressions []NamedExpression
	source      Node
	keep        bool
}

func NewMap(expressions []NamedExpression, child Node, keep bool) *Map {
	return &Map{expressions: expressions, source: child, keep: keep}
}

func (node *Map) Get(variables octosql.Variables) (RecordStream, error) {
	recordStream, err := node.source.Get(variables)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get record stream")
	}

	return &MappedStream{
		expressions: node.expressions,
		variables:   variables,
		source:      recordStream,
		keep:        node.keep,
	}, nil
}

type MappedStream struct {
	expressions []NamedExpression
	variables   octosql.Variables
	source      RecordStream
	keep        bool
}

func (stream *MappedStream) Next() (*Record, error) {
	srcRecord, err := stream.source.Next()
	if err != nil {
		if err == ErrEndOfStream {
			return nil, ErrEndOfStream
		}
		return nil, errors.Wrap(err, "couldn't get source record")
	}

	variables, err := stream.variables.MergeWith(srcRecord.AsVariables())
	if err != nil {
		return nil, errors.Wrap(err, "couldn't merge given variables with record variables")
	}

	fieldNames := make([]octosql.VariableName, 0)
	outValues := make(map[octosql.VariableName]interface{})
	for i := range stream.expressions {
		fieldNames = append(fieldNames, stream.expressions[i].Name())

		value, err := stream.expressions[i].ExpressionValue(variables)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't get expression %v", stream.expressions[i].Name())
		}

		if _, ok := value.([]Record); ok {
			return nil, errors.Errorf("multiple records ended up in one select field %+v", value)
		}

		if record, ok := value.(Record); ok {
			if len(record.Fields()) > 1 {
				return nil, errors.Errorf("multi field record ended up in one select field %+v", value)
			}
			outValues[stream.expressions[i].Name()] = record.Value(record.Fields()[0].Name)
			continue
		}
		outValues[stream.expressions[i].Name()] = value
	}

	if stream.keep {
		for _, name := range srcRecord.fieldNames {
			if _, ok := outValues[name]; !ok {
				fieldNames = append(fieldNames, name)
				outValues[name] = srcRecord.Value(name)
			}
		}
	}

	return NewRecord(fieldNames, outValues), nil
}