package chart

import (
	"fmt"
	"strconv"
)

func (v *JupyterConfigurableValues) ensureValidValues() error {
	floatVal, err := strconv.ParseFloat(v.CPUGuarantee, 64)
	if err != nil {
		return err
	}
	v.CPUGuarantee = fmt.Sprintf("%.1f", floatVal)

	floatVal, err = strconv.ParseFloat(v.CPULimit, 64)
	if err != nil {
		return err
	}
	v.CPULimit = fmt.Sprintf("%.1f", floatVal)

	_, err = strconv.ParseFloat(v.MemoryGuarantee, 64)
	if err != nil {
		return err
	}
	v.MemoryGuarantee = v.MemoryGuarantee + "G"

	_, err = strconv.ParseFloat(v.MemoryLimit, 64)
	if err != nil {
		return err
	}
	v.MemoryLimit = v.MemoryLimit + "G"

	return nil
}
