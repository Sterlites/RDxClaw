//go:build !linux

package tools

// scan is a stub for non-Linux platforms.
func (t *I2CTool) scan(_ map[string]interface{}) *ToolResult {
	return ErrorResult("I2C is only supported on Linux")
}

// readDevice is a stub for non-Linux platforms.
func (t *I2CTool) readDevice(_ map[string]interface{}) *ToolResult {
	return ErrorResult("I2C is only supported on Linux")
}

// writeDevice is a stub for non-Linux platforms.
func (t *I2CTool) writeDevice(_ map[string]interface{}) *ToolResult {
	return ErrorResult("I2C is only supported on Linux")
}
