package xray

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ComposeHelperChain composes two Xray profiles into one configuration. The
// primary outbound is configured to dial through the selected helper outbound.
// Only the helper outbound is imported: its inbound, routing, DNS, and logging
// settings do not replace the primary profile's settings.
//
// This is sequential proxy chaining, not bandwidth aggregation. It can improve
// reachability or route quality when the helper path is better, but it can also
// increase latency and cannot promise higher throughput.
func ComposeHelperChain(primaryConfig, helperConfig []byte, primaryOutboundTag, helperOutboundTag string) ([]byte, error) {
	primaryOutboundTag = strings.TrimSpace(primaryOutboundTag)
	helperOutboundTag = strings.TrimSpace(helperOutboundTag)
	if primaryOutboundTag == "" || helperOutboundTag == "" {
		return nil, errors.New("primary and helper outbound tags are required")
	}
	if primaryOutboundTag == helperOutboundTag {
		return nil, errors.New("primary and helper outbound tags must differ")
	}

	primary, err := decodeConfigObject(primaryConfig, "primary")
	if err != nil {
		return nil, err
	}
	helper, err := decodeConfigObject(helperConfig, "helper")
	if err != nil {
		return nil, err
	}
	primaryOutbounds, err := configOutbounds(primary, "primary")
	if err != nil {
		return nil, err
	}
	helperOutbounds, err := configOutbounds(helper, "helper")
	if err != nil {
		return nil, err
	}

	primaryTags := make(map[string]struct{}, len(primaryOutbounds))
	var primaryOutbound map[string]any
	for _, outbound := range primaryOutbounds {
		tag, _ := outbound["tag"].(string)
		if tag == "" {
			return nil, errors.New("every primary outbound must have a non-empty tag")
		}
		if _, exists := primaryTags[tag]; exists {
			return nil, fmt.Errorf("duplicate primary outbound tag %q", tag)
		}
		primaryTags[tag] = struct{}{}
		if tag == primaryOutboundTag {
			primaryOutbound = outbound
		}
	}
	if primaryOutbound == nil {
		return nil, fmt.Errorf("primary outbound %q was not found", primaryOutboundTag)
	}

	var helperOutbound map[string]any
	helperTags := make(map[string]struct{}, len(helperOutbounds))
	for _, outbound := range helperOutbounds {
		tag, _ := outbound["tag"].(string)
		if tag == "" {
			return nil, errors.New("every helper outbound must have a non-empty tag")
		}
		if _, exists := helperTags[tag]; exists {
			return nil, fmt.Errorf("duplicate helper outbound tag %q", tag)
		}
		helperTags[tag] = struct{}{}
		if tag == helperOutboundTag {
			helperOutbound = outbound
		}
	}
	if helperOutbound == nil {
		return nil, fmt.Errorf("helper outbound %q was not found", helperOutboundTag)
	}
	if _, exists := primaryTags[helperOutboundTag]; exists {
		return nil, fmt.Errorf("helper outbound tag %q conflicts with a primary outbound", helperOutboundTag)
	}
	if settings, ok := helperOutbound["proxySettings"].(map[string]any); ok {
		if strings.TrimSpace(stringValue(settings["tag"])) != "" {
			return nil, errors.New("selected helper outbound already chains through another outbound; nested helper chains are not supported")
		}
	}

	proxySettings, ok := primaryOutbound["proxySettings"].(map[string]any)
	if !ok {
		proxySettings = make(map[string]any)
	}
	if existing := strings.TrimSpace(stringValue(proxySettings["tag"])); existing != "" && existing != helperOutboundTag {
		return nil, fmt.Errorf("primary outbound %q already uses proxySettings tag %q", primaryOutboundTag, existing)
	}
	proxySettings["tag"] = helperOutboundTag
	primaryOutbound["proxySettings"] = proxySettings
	primaryOutbounds = append(primaryOutbounds, helperOutbound)
	encodedOutbounds, err := json.Marshal(primaryOutbounds)
	if err != nil {
		return nil, errors.New("could not encode composed Xray outbounds")
	}
	primary["outbounds"] = json.RawMessage(encodedOutbounds)
	result, err := json.Marshal(primary)
	if err != nil {
		return nil, errors.New("could not encode composed Xray config")
	}
	return result, nil
}

func decodeConfigObject(raw []byte, label string) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%s config is empty", label)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil || root == nil {
		return nil, fmt.Errorf("%s config must be a JSON object", label)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("%s config contains trailing JSON data", label)
	}
	return root, nil
}

func configOutbounds(root map[string]any, label string) ([]map[string]any, error) {
	items, ok := root["outbounds"].([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("%s config must contain a non-empty outbounds array", label)
	}
	out := make([]map[string]any, 0, len(items))
	for i, item := range items {
		ob, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s outbound %d must be an object", label, i)
		}
		out = append(out, ob)
	}
	return out, nil
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
