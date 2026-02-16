package logging

// StripANSI removes ANSI escape sequences from data.
func StripANSI(data []byte) []byte {
	result := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '[' {
			// Skip ESC [ ... final_byte sequence.
			i += 2
			for i < len(data) {
				b := data[i]
				i++
				// Final byte is in range 0x40-0x7E.
				if b >= 0x40 && b <= 0x7E {
					break
				}
			}
			continue
		}
		result = append(result, data[i])
		i++
	}
	return result
}
