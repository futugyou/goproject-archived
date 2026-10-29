package rrd

import (
	"fmt"
	"math"
)

type Value struct {
	Timestamp int64
	Value     float64
}

func (v *Value) String() string {
	return fmt.Sprintf("Value [value=%f, timestamp=%d]", v.Value, v.Timestamp)
}

type Variable interface {
	Fill(timestamps []int64, values []float64, start, end int64) Value
}

type AvgVariable struct{}

func (v *AvgVariable) Fill(timestamps []int64, values []float64, start, end int64) Value {
	var totalValue float64 = 0
	count := 0

	for i := len(values) - 1; i >= 0; i-- {
		if !math.IsNaN(values[i]) {
			count++
			totalValue += values[i]
		}
	}

	var result float64
	if !math.IsNaN(totalValue) && count > 0 {
		result = totalValue / float64(count)
	} else {
		result = math.NaN()
	}

	return Value{Timestamp: 0, Value: result}
}

type MinVariable struct{}

func (v *MinVariable) Fill(timestamps []int64, values []float64, start, end int64) Value {
	var timestamp int64 = 0
	value := math.NaN()
	for i := len(values) - 1; i >= 0; i-- {
		if !math.IsNaN(values[i]) && math.IsNaN(value) {
			timestamp = timestamps[i]
			value = values[i]
		} else if !math.IsNaN(values[i]) && value > values[i] {
			timestamp = timestamps[i]
			value = values[i]
		}
	}
	return Value{Timestamp: timestamp, Value: value}
}

type MaxVariable struct{}

func (v *MaxVariable) Fill(timestamps []int64, values []float64, start, end int64) Value {
	var timestamp int64 = 0
	value := math.NaN()
	for i := len(values) - 1; i >= 0; i-- {
		if !math.IsNaN(values[i]) && math.IsNaN(value) {
			timestamp = timestamps[i]
			value = values[i]
		} else if !math.IsNaN(values[i]) && value < values[i] {
			timestamp = timestamps[i]
			value = values[i]
		}
	}
	return Value{Timestamp: timestamp, Value: value}
}

type LastVariable struct{}

func (v *LastVariable) Fill(timestamps []int64, values []float64, start, end int64) Value {
	for i := len(values) - 1; i >= 0; i-- {
		if !math.IsNaN(values[i]) {
			return Value{Timestamp: timestamps[i], Value: values[i]}
		}
	}
	return Value{Timestamp: 0, Value: math.NaN()}
}

type FirstVariable struct{}

func (v *FirstVariable) Fill(timestamps []int64, values []float64, start, end int64) Value {
	for i := 0; i < len(values); i++ {
		if timestamps[i] > start && timestamps[i] < end && !math.IsNaN(values[i]) {
			return Value{Timestamp: timestamps[i], Value: values[i]}
		}
	}
	return Value{Timestamp: 0, Value: math.NaN()}
}

func Sum(x, y float64) float64 {
	if math.IsNaN(x) {
		return y
	}
	if math.IsNaN(y) {
		return x
	}
	return x + y
}

type TotalVariable struct{}

func (v *TotalVariable) Fill(timestamps []int64, values []float64, start, end int64) Value {
	value := math.NaN()
	for _, tempVal := range values {
		value = Sum(value, tempVal)
	}
	return Value{Timestamp: 0, Value: value * float64(timestamps[1]-timestamps[0])}
}

type ConsolidationFunction string

const (
	Average ConsolidationFunction = "AVERAGE"
	Min     ConsolidationFunction = "MIN"
	Max     ConsolidationFunction = "MAX"
	Last    ConsolidationFunction = "LAST"
	First   ConsolidationFunction = "FIRST"
	Total   ConsolidationFunction = "TOTAL"
)

func (cf ConsolidationFunction) GetVariable() Variable {
	switch cf {
	case Average:
		return &AvgVariable{}
	case Min:
		return &MinVariable{}
	case Max:
		return &MaxVariable{}
	case Last:
		return &LastVariable{}
	case First:
		return &LastVariable{}
	case Total:
		return &TotalVariable{}
	default:
		panic("unknown consolidation function")
	}
}
