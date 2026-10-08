package agentauth

// BuildCELChainMap transforms a ChainContext into a map[string]any suitable for CEL evaluation.
// Supports CEL expressions such as:
//
//	chain.root.type == "user" &&
//	chain.depth <= 2 &&
//	"tools:execute" in chain.effective_scopes &&
//	!chain.hops.exists(h, h.delegatee.endsWith("/unverified_crawler"))
func BuildCELChainMap(c *ChainContext) map[string]any {
	if c == nil {
		return map[string]any{
			"root": map[string]any{
				"principal": "",
				"type":      "",
			},
			"depth":            0,
			"effective_scopes": []string{},
			"caller_uri":       "",
			"hops":             []map[string]any{},
		}
	}

	hops := make([]map[string]any, 0, len(c.Hops))
	for _, h := range c.Hops {
		scopes := h.Scopes
		if scopes == nil {
			scopes = []string{}
		}
		hops = append(hops, map[string]any{
			"delegator": h.Delegator,
			"delegatee": h.Delegatee,
			"scopes":    scopes,
		})
	}

	effectiveScopes := c.EffectiveScopes
	if effectiveScopes == nil {
		effectiveScopes = []string{}
	}

	return map[string]any{
		"root": map[string]any{
			"principal": c.RootPrincipal,
			"type":      c.RootType,
		},
		"depth":            c.Depth,
		"effective_scopes": effectiveScopes,
		"caller_uri":       c.CallerURI,
		"hops":             hops,
	}
}
