package utils

func ParseInt32(raw interface{}) (int32, bool) {
	switch v := raw.(type) {
	case int32:
		return v, true
	case int:
		return int32(v), true
	case int64:
		return int32(v), true
	case float32:
		return int32(v), true
	case float64:
		return int32(v), true
	}
	return 0, false
}

func ParseUint32(raw interface{}) (uint32, bool) {
	switch v := raw.(type) {
	case uint32:
		return v, true
	case uint64:
		return uint32(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int32:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case float32:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	}
	return 0, false
}
