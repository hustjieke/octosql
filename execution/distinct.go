package execution

import (
	"github.com/cube2222/octosql"

	"context"

	"github.com/mitchellh/hashstructure"
	"github.com/pkg/errors"
)

type Distinct struct {
	child Node
}

func NewDistinct(child Node) *Distinct {
	return &Distinct{child: child}
}

func (node *Distinct) Get(ctx context.Context, variables octosql.Variables) (RecordStream, error) {
	stream, err := node.child.Get(ctx, variables)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get stream for child node in distinct")
	}

	return &DistinctStream{
		stream:    stream,
		variables: variables,
		records:   newRecordSet(),
	}, nil
}

type DistinctStream struct {
	stream    RecordStream
	variables octosql.Variables
	records   *recordSet
}

func (ds *DistinctStream) Close() error {
	err := ds.stream.Close()
	if err != nil {
		return errors.Wrap(err, "Couldn't close underlying stream")
	}

	return nil
}

func (ds *DistinctStream) Next(ctx context.Context) (*Record, error) {
	for {
		record, err := ds.stream.Next(ctx)
		if err != nil {
			if err == ErrEndOfStream {
				return nil, ErrEndOfStream
			}
			return nil, errors.Wrap(err, "couldn't get record from stream in DistinctStream")
		}

		already, err := ds.records.Has(record)

		if err != nil {
			return nil, errors.Wrap(err, "couldn't access the record set")
		}

		if !already {
			_, err := ds.records.Insert(record)

			if err != nil {
				return nil, errors.Wrap(err, "couldn't access the record set")
			}

			return record, nil
		}
	}
}

type recordSet struct {
	set map[uint64][]*Record
}

func newRecordSet() *recordSet {
	return &recordSet{
		set: make(map[uint64][]*Record),
	}
}

func (rs *recordSet) Has(r *Record) (bool, error) {
	hash, err := HashRecord(r)
	if err != nil {
		return false, errors.Wrap(err, "couldn't get hash of record")
	}

	for i := range rs.set[hash] {
		if r.Equal(rs.set[hash][i]) {
			return true, nil
		}
	}

	return false, nil
}

func (rs *recordSet) Insert(r *Record) (bool, error) {
	hash, err := HashRecord(r)
	if err != nil {
		return false, errors.Wrap(err, "couldn't get hash of record")
	}

	already, err := rs.Has(r)
	if err != nil {
		return false, errors.Wrap(err, "couldn't find out if record already in set")
	}
	if !already {
		rs.set[hash] = append(rs.set[hash], r)
		return true, nil
	}

	return false, nil
}

func HashRecord(rec *Record) (uint64, error) {
	return hashstructure.Hash(rec.data, nil)
}
