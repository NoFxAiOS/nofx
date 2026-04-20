package trader

import "reflect"

func reflectValue(v interface{}) reflect.Value {
	rv := reflect.ValueOf(v)
	for rv.IsValid() && rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	return rv
}

func setFloatFromField(v reflect.Value, field string, dest *float64, source map[string]string, sourceKey, sourceValue string) {
	if !v.IsValid() || v.Kind() != reflect.Struct || dest == nil || source == nil {
		return
	}
	f := v.FieldByName(field)
	if !f.IsValid() || !f.CanInterface() {
		return
	}
	var value float64
	switch f.Kind() {
	case reflect.Float32, reflect.Float64:
		value = f.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value = float64(f.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value = float64(f.Uint())
	default:
		return
	}
	if value > 0 {
		*dest = value
		source[sourceKey] = sourceValue
	}
}

func setSnapshotMapValue(m map[string]float64, key string, dest *float64, source map[string]string, sourceValue string) {
	if m == nil || dest == nil || source == nil {
		return
	}
	if v, ok := m[key]; ok && v > 0 {
		*dest = v
		source[key] = sourceValue
	}
}
