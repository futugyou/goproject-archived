package rrd

import (
	"fmt"
	"math"
)

type DatasourceType string

const (
	GAUGE    DatasourceType = "GAUGE"
	COUNTER  DatasourceType = "COUNTER"
	DERIVE   DatasourceType = "DERIVE"
	ABSOLUTE DatasourceType = "ABSOLUTE"
)

type DataSourceExpression struct {
	Name      string
	Type      DatasourceType
	Heartbeat int64
	MinValue  float64
	MaxValue  float64
}

func NewDataSourceExpression(name string, dstype DatasourceType, heartbeat int64, minValue float64, maxValue float64) *DataSourceExpression {
	//  TODO: check parameter

	return &DataSourceExpression{
		Name:      name,
		Type:      dstype,
		Heartbeat: heartbeat,
		MinValue:  minValue,
		MaxValue:  maxValue,
	}
}

func FormatDouble(x float64, nanString string) string {
	if math.IsNaN(x) {
		return nanString
	}

	return fmt.Sprint(x)
}

func (d *DataSourceExpression) String() string {
	return fmt.Sprintf("DS:%s:%s:%d:%s:%s", d.Name, d.Type, d.Heartbeat, FormatDouble(d.MinValue, "U"), FormatDouble(d.MaxValue, "U"))
}
