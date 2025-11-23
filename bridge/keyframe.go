package bridge

func IsKeyframe(data []byte, codec string) bool {
	if codec == "h264" {
		return isKeyframeH264(data)
	} else if codec == "hevc" {
		return isKeyframeHEVC(data)
	}
	return true // Default to true if unknown codec to avoid blocking
}

func isKeyframeH264(data []byte) bool {
	i := 0
	for i < len(data)-4 {
		if data[i] == 0x00 && data[i+1] == 0x00 {
			if data[i+2] == 0x01 {
				// 00 00 01
				nalUnitType := data[i+3] & 0x1f
				if nalUnitType == 5 {
					return true
				}
				i += 3
			} else if data[i+2] == 0x00 && data[i+3] == 0x01 {
				// 00 00 00 01
				nalUnitType := data[i+4] & 0x1f
				if nalUnitType == 5 {
					return true
				}
				i += 4
			} else {
				i++
			}
		} else {
			i++
		}
	}
	return false
}

func isKeyframeHEVC(data []byte) bool {
	i := 0
	for i < len(data)-4 {
		if data[i] == 0x00 && data[i+1] == 0x00 {
			if data[i+2] == 0x01 {
				// 00 00 01
				nalStart := i + 3
				nalUnitType := (data[nalStart] >> 1) & 0x3f
				if isHEVCKeyframe(nalUnitType) {
					return true
				}
				i += 3
			} else if data[i+2] == 0x00 && data[i+3] == 0x01 {
				// 00 00 00 01
				nalStart := i + 4
				nalUnitType := (data[nalStart] >> 1) & 0x3f
				if isHEVCKeyframe(nalUnitType) {
					return true
				}
				i += 4
			} else {
				i++
			}
		} else {
			i++
		}
	}
	return false
}

func isHEVCKeyframe(nalUnitType byte) bool {
	// IDR_W_RADL=19, IDR_N_LP=20, CRA_NUT=21, BLA_W_LP=16, BLA_W_RADL=17, BLA_N_LP=18
	// Python code checked [16, 17, 18, 19, 20]
	return nalUnitType >= 16 && nalUnitType <= 20
}
