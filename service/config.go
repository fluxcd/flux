package service

import (
	"encoding/json"
)

type NotifierConfig struct {
	HookURL         string `json:"hookURL" yaml:"hookURL"`
	Username        string `json:"username" yaml:"username"`
	ReleaseTemplate string `json:"releaseTemplate" yaml:"releaseTemplate"`
}

type InstanceConfig struct {
	Slack NotifierConfig `json:"slack" yaml:"slack"`
}

type untypedConfig map[string]interface{}

func (uc untypedConfig) toInstanceConfig() (InstanceConfig, error) {
	bytes, err := json.Marshal(uc)
	if err != nil {
		return InstanceConfig{}, err
	}
	var uic InstanceConfig
	if err := json.Unmarshal(bytes, &uic); err != nil {
		return InstanceConfig{}, err
	}
	return uic, nil
}

func (uic InstanceConfig) toUntypedConfig() (untypedConfig, error) {
	bytes, err := json.Marshal(uic)
	if err != nil {
		return nil, err
	}
	var uc untypedConfig
	if err := json.Unmarshal(bytes, &uc); err != nil {
		return nil, err
	}
	return uc, nil
}

type ConfigPatch map[string]interface{}

func (uic InstanceConfig) Patch(cp ConfigPatch) (InstanceConfig, error) {
	// Convert the strongly-typed config into an untyped form that's easier to patch
	uc, err := uic.toUntypedConfig()
	if err != nil {
		return InstanceConfig{}, err
	}

	applyPatch(uc, cp)

	// If the modifications detailed by the patch have resulted in JSON which
	// doesn't meet the config schema it will be caught here
	return uc.toInstanceConfig()
}

func applyPatch(uc untypedConfig, cp ConfigPatch) {
	for key, value := range cp {
		switch value := value.(type) {
		case nil:
			delete(uc, key)
		case map[string]interface{}:
			if uc[key] == nil {
				uc[key] = make(map[string]interface{})
			}
			if uc, ok := uc[key].(map[string]interface{}); ok {
				applyPatch(uc, value)
			}
		default:
			// Remaining types; []interface{}, bool, float64 & string
			// Note that we only support replacing arrays in their entirety
			// as there is no way to address subelements for removal or update
			uc[key] = value
		}
	}
}
