/*
 * Copyright (c) 2026 SECOM CO., LTD. All Rights reserved.
 *
 * SPDX-License-Identifier: BSD-2-Clause
 */

package util

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/veraison/go-cose"
)

func RenderCBORPretty(decoded any) (string, error) {
	normalised, err := normaliseCBORForJSON(decoded)
	if err != nil {
		return "", err
	}

	pretty, err := json.MarshalIndent(normalised, "", "  ")
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}

func normaliseCBORForJSON(value any) (any, error) {
	switch v := value.(type) {
	case []any:
		out := make([]any, len(v))
		for i, elem := range v {
			norm, err := normaliseCBORForJSON(elem)
			if err != nil {
				return nil, err
			}
			out[i] = norm
		}
		return out, nil
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		out := make(map[string]any, len(v))
		for _, k := range keys {
			norm, err := normaliseCBORForJSON(v[k])
			if err != nil {
				return nil, err
			}
			out[k] = norm
		}
		return out, nil
	case map[any]any:
		type entry struct {
			key string
			val any
		}

		entries := make([]entry, 0, len(v))
		for key, val := range v {
			keyStr := stringifyCBORKey(key)
			norm, err := normaliseCBORForJSON(val)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entry{key: keyStr, val: norm})
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].key < entries[j].key
		})

		out := make(map[string]any, len(entries))
		for _, e := range entries {
			out[e.key] = e.val
		}
		return out, nil
	case []byte:
		return fmt.Sprintf("h'%x'", v), nil
	case cbor.Tag:
		content, err := normaliseCBORForJSON(v.Content)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"_cborTag": v.Number,
			"content":  content,
		}, nil
	default:
		return v, nil
	}
}

func stringifyCBORKey(key any) string {
	switch k := key.(type) {
	case string:
		return k
	case fmt.Stringer:
		return k.String()
	case []byte:
		return fmt.Sprintf("h'%x'", k)
	default:
		return fmt.Sprint(k)
	}
}

type CBORDiagFormattable interface {
	CBORDiagString(indent int) string
}

// utility function to format a list of items with comma seperation
type DiagList[T CBORDiagFormattable] []T

func (l DiagList[T]) CBORDiagString(indent int) string {
	var formattedString []string
	for _, v := range l {
		formattedString = append(formattedString, v.CBORDiagString(indent))
	}
	return fmt.Sprintf("[%s]", strings.Join(formattedString, ", "))
}

type BytesHexMax32 []byte

func (b BytesHexMax32) String() string {
	return b.CBORDiagString(0)
}

func (b BytesHexMax32) CBORDiagString(indent int) string {
	l := len(b)
	if l > 32 {
		return fmt.Sprintf("h'%s'/.../", strings.ToUpper(hex.EncodeToString(b[:32]))) // truncate
	}
	return fmt.Sprintf("h'%s'", strings.ToUpper(hex.EncodeToString(b)))
}

type DiagString string

func (d DiagString) CBORDiagString(indent int) string {
	return fmt.Sprintf("\"%s\"", string(d))
}

func PrintCOSEKey(key *cose.Key) string {
	if key == nil {
		return "null"
	}

	var b strings.Builder
	b.WriteByte('{')
	fieldCount := 0

	appendField := func(name, value string) {
		if value == "" {
			return
		}
		if fieldCount > 0 {
			b.WriteByte(',')
		}
		fieldCount++
		b.WriteByte('"')
		b.WriteString(name)
		b.WriteString(`":`)
		b.WriteString(value)
	}
	appendDiagString := func(name, value string) {
		if value == "" {
			return
		}
		appendField(name, DiagString(value).CBORDiagString(0))
	}
	appendBytes := func(name string, value []byte) {
		if len(value) == 0 {
			return
		}
		appendField(name, BytesHexMax32(value).CBORDiagString(0))
	}

	appendDiagString("kty", key.Type.String())

	var skipParamLabels map[any]struct{}
	switch key.Type {
	case cose.KeyTypeEC2:
		crv, x, y, d := key.EC2()
		appendDiagString("crv", crv.String())
		appendBytes("x", x)
		appendBytes("y", y)
		appendBytes("d", d)
		skipParamLabels = map[any]struct{}{
			cose.KeyLabelEC2Curve: {},
			cose.KeyLabelEC2X:     {},
			cose.KeyLabelEC2Y:     {},
			cose.KeyLabelEC2D:     {},
		}
	case cose.KeyTypeOKP:
		crv, x, d := key.OKP()
		appendDiagString("crv", crv.String())
		appendBytes("x", x)
		appendBytes("d", d)
		skipParamLabels = map[any]struct{}{
			cose.KeyLabelOKPCurve: {},
			cose.KeyLabelOKPX:     {},
			cose.KeyLabelOKPD:     {},
		}
	case cose.KeyTypeSymmetric:
		appendBytes("k", key.Symmetric())
	}

	appendBytes("kid", key.ID)
	if key.Algorithm != cose.AlgorithmReserved {
		appendDiagString("alg", key.Algorithm.String())
	}
	if len(key.Ops) > 0 {
		ops := make([]string, len(key.Ops))
		for i, op := range key.Ops {
			ops[i] = DiagString(op.String()).CBORDiagString(0)
		}
		appendField("key_ops", "["+strings.Join(ops, ",")+"]")
	}
	appendBytes("base_iv", key.BaseIV)

	if len(key.Params) > 0 {
		type entry struct {
			label string
			value string
		}
		entries := make([]entry, 0, len(key.Params))
		for label, value := range key.Params {
			if _, ok := skipParamLabels[label]; ok {
				continue
			}
			entries = append(entries, entry{
				label: stringifyCBORKey(label),
				value: stringifyCBORValue(value),
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].label < entries[j].label
		})
		for _, e := range entries {
			appendField(e.label, e.value)
		}
	}

	b.WriteByte('}')
	return b.String()
}

func stringifyCBORValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return DiagString(v).CBORDiagString(0)
	case []byte:
		return BytesHexMax32(v).CBORDiagString(0)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case fmt.Stringer:
		return DiagString(v.String()).CBORDiagString(0)
	default:
		return fmt.Sprint(v)
	}
}
