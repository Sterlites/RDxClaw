//go:build !linux

package tools

// transfer is a stub for non-Linux platforms.
func (t *SPITool) transfer(_ map[string]interface{}) *ToolResult {
	return ErrorResult("SPI is only supported on Linux")
}

// readDevice is a stub for non-Linux platforms.
func (t *SPITool) readDevice(_ map[string]interface{}) *ToolResult {
	return ErrorResult("SPI is only supported on Linux")
}
