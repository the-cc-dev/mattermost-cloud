package model

import rds "github.com/aws/aws-sdk-go/service/rds"

// Tags ...
type Tags []*struct {
	Key   string
	Value string
}

// RDS ...
func (t *Tags) RDS() []*rds.Tag {
	rdsTags := make([]*rds.Tag, len(*t))
	for k, v := range *t {
		rdsTags[k] = &rds.Tag{Key: &v.Key, Value: &v.Value}
	}
	return rdsTags
}
