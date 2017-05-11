package mapping

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/omniscale/imposm3/element"

	"gopkg.in/yaml.v2"
)

type Mapping struct {
	Tables            Tables            `yaml:"tables"`
	GeneralizedTables GeneralizedTables `yaml:"generalized_tables"`
	Tags              Tags              `yaml:"tags"`
	Areas             Areas             `yaml:"areas"`
	// SingleIdSpace mangles the overlapping node/way/relation IDs
	// to be unique (nodes positive, ways negative, relations negative -1e17)
	SingleIdSpace bool `yaml:"use_single_id_space"`
}

type Column struct {
	Name       string                 `yaml:"name"`
	Key        Key                    `yaml:"key"`
	Keys       []Key                  `yaml:"keys"`
	Type       string                 `yaml:"type"`
	Args       map[string]interface{} `yaml:"args"`
	FromMember bool                   `yaml:"from_member"`
}

func (c *Column) ColumnType() *ColumnType {
	if columnType, ok := AvailableColumnTypes[c.Type]; ok {
		if columnType.MakeFunc != nil {
			makeValue, err := columnType.MakeFunc(c.Name, columnType, *c)
			if err != nil {
				log.Print(err)
				return nil
			}
			columnType = ColumnType{columnType.Name, columnType.GoType, makeValue, nil, nil, columnType.FromMember}
		}
		columnType.FromMember = c.FromMember
		return &columnType
	}
	return nil
}

type Tables map[string]*Table
type Table struct {
	Name          string
	Type          TableType             `yaml:"type"`
	Mapping       KeyValues             `yaml:"mapping"`
	Mappings      map[string]SubMapping `yaml:"mappings"`
	TypeMappings  TypeMappings          `yaml:"type_mappings"`
	Columns       []*Column             `yaml:"columns"`
	OldFields     []*Column             `yaml:"fields"`
	Filters       *Filters              `yaml:"filters"`
	RelationTypes []string              `yaml:"relation_types"`
}

type GeneralizedTables map[string]*GeneralizedTable
type GeneralizedTable struct {
	Name            string
	SourceTableName string  `yaml:"source"`
	Tolerance       float64 `yaml:"tolerance"`
	SqlFilter       string  `yaml:"sql_filter"`
}

type Filters struct {
	ExcludeTags *[][]string `yaml:"exclude_tags"`
}

type Areas struct {
	AreaTags   []Key `yaml:"area_tags"`
	LinearTags []Key `yaml:"linear_tags"`
}

type Tags struct {
	LoadAll bool  `yaml:"load_all"`
	Exclude []Key `yaml:"exclude"`
	Include []Key `yaml:"include"`
}

type orderedValue struct {
	value Value
	order int
}

type KeyValues map[Key][]orderedValue

func (kv *KeyValues) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if *kv == nil {
		*kv = make(map[Key][]orderedValue)
	}
	slice := yaml.MapSlice{}
	err := unmarshal(&slice)
	if err != nil {
		return err
	}
	order := 0
	for _, item := range slice {
		k, ok := item.Key.(string)
		if !ok {
			return fmt.Errorf("mapping key '%s' not a string", k)
		}
		values, ok := item.Value.([]interface{})
		if !ok {
			return fmt.Errorf("mapping key '%s' not a string", k)
		}
		for _, v := range values {
			if v, ok := v.(string); ok {
				(*kv)[Key(k)] = append((*kv)[Key(k)], orderedValue{value: Value(v), order: order})
			} else {
				return fmt.Errorf("mapping value '%s' not a string", v)
			}
			order += 1
		}
	}
	return nil
}

type SubMapping struct {
	Mapping KeyValues
}

type TypeMappings struct {
	Points      KeyValues `yaml:"points"`
	LineStrings KeyValues `yaml:"linestrings"`
	Polygons    KeyValues `yaml:"polygons"`
}

type ElementFilter func(tags element.Tags, key Key, closed bool) bool

type orderedDestTable struct {
	DestTable
	order int
}

type TagTableMapping map[Key]map[Value][]orderedDestTable

func (tt TagTableMapping) addFromMapping(mapping KeyValues, table DestTable) {
	for key, vals := range mapping {
		for _, v := range vals {
			vals, ok := tt[key]
			tbl := orderedDestTable{DestTable: table, order: v.order}
			if ok {
				vals[v.value] = append(vals[v.value], tbl)
			} else {
				tt[key] = make(map[Value][]orderedDestTable)
				tt[key][v.value] = append(tt[key][v.value], tbl)
			}
		}
	}
}

func (tt TagTableMapping) asTagMap() tagMap {
	result := make(tagMap)
	for k, vals := range tt {
		result[k] = make(map[Value]struct{})
		for v := range vals {
			result[k][v] = struct{}{}
		}
	}
	return result
}

type DestTable struct {
	Name       string
	SubMapping string
}

type TableType string

func (tt *TableType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "":
		return errors.New("missing table type")
	case `"point"`:
		*tt = PointTable
	case `"linestring"`:
		*tt = LineStringTable
	case `"polygon"`:
		*tt = PolygonTable
	case `"geometry"`:
		*tt = GeometryTable
	case `"relation"`:
		*tt = RelationTable
	case `"relation_member"`:
		*tt = RelationMemberTable
	default:
		return errors.New("unknown type " + string(data))
	}
	return nil
}

const (
	PolygonTable        TableType = "polygon"
	LineStringTable     TableType = "linestring"
	PointTable          TableType = "point"
	GeometryTable       TableType = "geometry"
	RelationTable       TableType = "relation"
	RelationMemberTable TableType = "relation_member"
)

func NewMapping(filename string) (*Mapping, error) {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	mapping := Mapping{}
	err = yaml.Unmarshal(f, &mapping)
	if err != nil {
		return nil, err
	}

	err = mapping.prepare()
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

func (m *Mapping) prepare() error {
	for name, t := range m.Tables {
		t.Name = name
		if t.OldFields != nil {
			// todo deprecate 'fields'
			t.Columns = t.OldFields
		}
	}

	for name, t := range m.GeneralizedTables {
		t.Name = name
	}
	return nil
}

func (m *Mapping) mappings(tableType TableType, mappings TagTableMapping) {
	for name, t := range m.Tables {
		if t.Type != GeometryTable && t.Type != tableType {
			continue
		}
		mappings.addFromMapping(t.Mapping, DestTable{Name: name})

		for subMappingName, subMapping := range t.Mappings {
			mappings.addFromMapping(subMapping.Mapping, DestTable{Name: name, SubMapping: subMappingName})
		}

		switch tableType {
		case PointTable:
			mappings.addFromMapping(t.TypeMappings.Points, DestTable{Name: name})
		case LineStringTable:
			mappings.addFromMapping(t.TypeMappings.LineStrings, DestTable{Name: name})
		case PolygonTable:
			mappings.addFromMapping(t.TypeMappings.Polygons, DestTable{Name: name})
		}
	}
}

func (m *Mapping) tables(tableType TableType) map[string]*TableSpec {
	result := make(map[string]*TableSpec)
	for name, t := range m.Tables {
		if t.Type == tableType || t.Type == GeometryTable {
			result[name] = t.TableSpec()
		}
	}
	return result
}

func (m *Mapping) extraTags(tableType TableType, tags map[Key]bool) {
	for _, t := range m.Tables {
		if t.Type != tableType && t.Type != GeometryTable {
			continue
		}

		for _, col := range t.Columns {
			if col.Key != "" {
				tags[col.Key] = true
			}
			for _, k := range col.Keys {
				tags[k] = true
			}
		}

		if t.Filters != nil && t.Filters.ExcludeTags != nil {
			for _, keyVal := range *t.Filters.ExcludeTags {
				tags[Key(keyVal[0])] = true
			}
		}

		if tableType == PolygonTable || tableType == RelationTable || tableType == RelationMemberTable {
			if t.RelationTypes != nil {
				tags["type"] = true
			}
		}
	}
	for _, k := range m.Tags.Include {
		tags[k] = true
	}

	// always include area tag for closed-way handling
	tags["area"] = true
}

type tableElementFilters map[string][]ElementFilter

func (m *Mapping) addTypedFilters(tableType TableType, filters tableElementFilters) {
	var areaTags map[Key]struct{}
	var linearTags map[Key]struct{}
	if m.Areas.AreaTags != nil {
		areaTags = make(map[Key]struct{})
		for _, tag := range m.Areas.AreaTags {
			areaTags[tag] = struct{}{}
		}
	}
	if m.Areas.LinearTags != nil {
		linearTags = make(map[Key]struct{})
		for _, tag := range m.Areas.LinearTags {
			linearTags[tag] = struct{}{}
		}
	}

	for name, t := range m.Tables {
		if t.Type != GeometryTable && t.Type != tableType {
			continue
		}
		if t.Type == LineStringTable && areaTags != nil {
			f := func(tags element.Tags, key Key, closed bool) bool {
				if closed {
					if tags["area"] == "yes" {
						return false
					}
					if tags["area"] != "no" {
						if _, ok := areaTags[key]; ok {
							return false
						}
					}
				}
				return true
			}
			filters[name] = append(filters[name], f)
		}
		if t.Type == PolygonTable && linearTags != nil {
			f := func(tags element.Tags, key Key, closed bool) bool {
				if closed && tags["area"] == "no" {
					return false
				}
				if tags["area"] != "yes" {
					if _, ok := linearTags[key]; ok {
						return false
					}
				}
				return true
			}
			filters[name] = append(filters[name], f)
		}
	}
}

func (m *Mapping) addRelationFilters(tableType TableType, filters tableElementFilters) {
	for name, t := range m.Tables {
		if t.RelationTypes != nil {
			relTypes := t.RelationTypes // copy loop var for closure
			f := func(tags element.Tags, key Key, closed bool) bool {
				if v, ok := tags["type"]; ok {
					for _, rtype := range relTypes {
						if v == rtype {
							return true
						}
					}
				}
				return false
			}
			filters[name] = append(filters[name], f)
		} else {
			if t.Type == PolygonTable {
				// standard mulipolygon handling (boundary and land_area are for backwards compatibility)
				f := func(tags element.Tags, key Key, closed bool) bool {
					if v, ok := tags["type"]; ok {
						if v == "multipolygon" || v == "boundary" || v == "land_area" {
							return true
						}
					}
					return false
				}
				filters[name] = append(filters[name], f)
			}
		}
	}
}

func (m *Mapping) addFilters(filters tableElementFilters) {
	for name, t := range m.Tables {
		if t.Filters == nil {
			continue
		}
		if t.Filters.ExcludeTags != nil {
			for _, filterKeyVal := range *t.Filters.ExcludeTags {
				f := func(tags element.Tags, key Key, closed bool) bool {
					if v, ok := tags[filterKeyVal[0]]; ok {
						if filterKeyVal[1] == "__any__" || v == filterKeyVal[1] {
							return false
						}
					}
					return true
				}
				filters[name] = append(filters[name], f)
			}
		}
	}
}
