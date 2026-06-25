package core

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type YamlProperty struct {
	Key   string
	Value SkillYamlNode
}

type SkillYamlNode interface {
	WriteTo(enc *json.Encoder) error
}

type SkillYamlObjectNode struct {
	Properties []PropertyPair
}

type PropertyPair struct {
	Key   string
	Value SkillYamlNode
}

func (n *SkillYamlObjectNode) WriteTo(enc *json.Encoder) error {
	objMap := make(map[string]interface{})
	for _, prop := range n.Properties {
		var buf bytes.Buffer
		tempEnc := json.NewEncoder(&buf)
		if err := prop.Value.WriteTo(tempEnc); err != nil {
			return err
		}
		objMap[prop.Key] = json.RawMessage(bytes.TrimSpace(buf.Bytes()))
	}
	return enc.Encode(objMap)
}

type SkillYamlArrayNode struct {
	Items []SkillYamlNode
}

func (n *SkillYamlArrayNode) WriteTo(enc *json.Encoder) error {
	var arr []json.RawMessage
	for _, item := range n.Items {
		var buf bytes.Buffer
		tempEnc := json.NewEncoder(&buf)
		if err := item.WriteTo(tempEnc); err != nil {
			return err
		}
		arr = append(arr, json.RawMessage(bytes.TrimSpace(buf.Bytes())))
	}
	return enc.Encode(arr)
}

type SkillYamlScalarNode struct {
	Value interface{}
}

func (n *SkillYamlScalarNode) WriteTo(enc *json.Encoder) error {
	if n.Value == nil {
		return enc.Encode(nil)
	}

	switch v := n.Value.(type) {
	case bool, int, int64, float64, string:
		return enc.Encode(v)
	default:
		return enc.Encode(fmt.Sprintf("%v", v))
	}
}
