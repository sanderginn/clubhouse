package links

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

type RecipeData struct {
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Image         string         `json:"image,omitempty"`
	Ingredients   []string       `json:"ingredients,omitempty"`
	Instructions  []string       `json:"instructions,omitempty"`
	PrepTime      string         `json:"prep_time,omitempty"`
	CookTime      string         `json:"cook_time,omitempty"`
	TotalTime     string         `json:"total_time,omitempty"`
	Yield         string         `json:"yield,omitempty"`
	Author        string         `json:"author,omitempty"`
	DatePublished string         `json:"date_published,omitempty"`
	Cuisine       string         `json:"cuisine,omitempty"`
	Category      string         `json:"category,omitempty"`
	NutritionInfo *NutritionInfo `json:"nutrition,omitempty"`
}

type NutritionInfo struct {
	Calories string `json:"calories,omitempty"`
	Servings string `json:"servings,omitempty"`
}

var durationPattern = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

func ParseRecipeSchema(body []byte) (*RecipeData, error) {
	if len(body) == 0 {
		return nil, nil
	}

	scripts, err := extractJSONLDScripts(body)
	if err != nil {
		return nil, err
	}

	var firstErr error
	for _, script := range scripts {
		if strings.TrimSpace(script) == "" {
			continue
		}
		var payload interface{}
		if err := json.Unmarshal([]byte(script), &payload); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if recipe := findRecipeInJSONLD(payload); recipe != nil {
			return recipe, nil
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

func ExtractRecipeFromHTML(body []byte, hostname string) (*RecipeData, error) {
	_ = hostname
	if len(body) == 0 {
		return nil, nil
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if recipe := extractRecipeFromMicrodata(doc); recipe != nil {
		return recipe, nil
	}

	recipe := extractRecipeFromHeuristics(doc, body)
	if recipe == nil {
		return nil, nil
	}
	return recipe, nil
}

func FormatDuration(isoDuration string) string {
	trimmed := strings.TrimSpace(isoDuration)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "P") {
		return trimmed
	}
	match := durationPattern.FindStringSubmatch(upper)
	if match == nil {
		return trimmed
	}

	parts := make([]string, 0, 4)
	if value := strings.TrimLeft(match[1], "0"); value != "" {
		parts = append(parts, fmt.Sprintf("%sd", value))
	}
	if value := strings.TrimLeft(match[2], "0"); value != "" {
		parts = append(parts, fmt.Sprintf("%sh", value))
	}
	if value := strings.TrimLeft(match[3], "0"); value != "" {
		parts = append(parts, fmt.Sprintf("%sm", value))
	}
	if value := strings.TrimLeft(match[4], "0"); value != "" {
		parts = append(parts, fmt.Sprintf("%ss", value))
	}
	if len(parts) == 0 {
		return trimmed
	}
	return strings.Join(parts, " ")
}

func parseRecipeIfPresent(body []byte, hostname string) *RecipeData {
	if recipe, err := ParseRecipeSchema(body); err == nil && recipe != nil {
		return recipe
	}
	if isKnownRecipeSite(hostname) {
		if recipe, err := ExtractRecipeFromHTML(body, hostname); err == nil {
			return recipe
		}
	}
	return nil
}

func isKnownRecipeSite(hostname string) bool {
	host := strings.ToLower(strings.TrimSpace(hostname))
	if host == "" {
		return false
	}

	known := []string{
		"allrecipes.com",
		"epicurious.com",
		"foodnetwork.com",
		"bonappetit.com",
		"seriouseats.com",
		"simplyrecipes.com",
		"tasty.co",
	}
	for _, site := range known {
		if host == site || strings.HasSuffix(host, "."+site) {
			return true
		}
	}
	return false
}

func extractJSONLDScripts(body []byte) ([]string, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	scripts := []string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "script") {
			var scriptType string
			for _, attr := range n.Attr {
				if strings.EqualFold(attr.Key, "type") {
					scriptType = strings.ToLower(strings.TrimSpace(attr.Val))
					break
				}
			}
			if strings.Contains(scriptType, "ld+json") {
				var builder strings.Builder
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					if child.Type == html.TextNode {
						builder.WriteString(child.Data)
					}
				}
				scripts = append(scripts, builder.String())
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return scripts, nil
}

func findRecipeInJSONLD(payload interface{}) *RecipeData {
	switch value := payload.(type) {
	case map[string]interface{}:
		return recipeFromMap(value)
	case []interface{}:
		for _, item := range value {
			if recipe := findRecipeInJSONLD(item); recipe != nil {
				return recipe
			}
		}
	}
	return nil
}

func recipeFromMap(payload map[string]interface{}) *RecipeData {
	if payload == nil {
		return nil
	}
	if graph, ok := payload["@graph"]; ok {
		if recipe := findRecipeInJSONLD(graph); recipe != nil {
			return recipe
		}
	}
	if mainEntity, ok := payload["mainEntity"]; ok {
		if recipe := findRecipeInJSONLD(mainEntity); recipe != nil {
			return recipe
		}
	}
	if mainEntityOfPage, ok := payload["mainEntityOfPage"]; ok {
		if recipe := findRecipeInJSONLD(mainEntityOfPage); recipe != nil {
			return recipe
		}
	}
	if item, ok := payload["item"]; ok {
		if recipe := findRecipeInJSONLD(item); recipe != nil {
			return recipe
		}
	}

	if !isRecipeType(payload["@type"]) {
		return nil
	}

	recipe := &RecipeData{}
	recipe.Name = parseString(payload["name"])
	recipe.Description = parseString(payload["description"])
	recipe.Image = parseImage(payload["image"])
	recipe.Ingredients = parseStringSlice(payload["recipeIngredient"], payload["ingredients"])
	recipe.Instructions = parseInstructions(payload["recipeInstructions"])
	recipe.PrepTime = FormatDuration(parseString(payload["prepTime"]))
	recipe.CookTime = FormatDuration(parseString(payload["cookTime"]))
	recipe.TotalTime = FormatDuration(parseString(payload["totalTime"]))
	recipe.Yield = parseString(payload["recipeYield"])
	recipe.Author = parseAuthor(payload["author"])
	recipe.DatePublished = parseString(payload["datePublished"])
	recipe.Cuisine = parseString(payload["recipeCuisine"])
	recipe.Category = parseString(payload["recipeCategory"])
	recipe.NutritionInfo = parseNutrition(payload["nutrition"])

	if recipe.Name == "" && len(recipe.Ingredients) == 0 && len(recipe.Instructions) == 0 {
		return nil
	}

	return recipe
}

func extractRecipeFromMicrodata(doc *html.Node) *RecipeData {
	if doc == nil {
		return nil
	}

	var recipeNode *html.Node
	var findRecipeNode func(*html.Node)
	findRecipeNode = func(n *html.Node) {
		if n.Type == html.ElementNode {
			var itemType string
			var hasScope bool
			for _, attr := range n.Attr {
				if strings.EqualFold(attr.Key, "itemscope") {
					hasScope = true
				}
				if strings.EqualFold(attr.Key, "itemtype") {
					itemType = strings.ToLower(attr.Val)
				}
			}
			if hasScope && strings.Contains(itemType, "recipe") {
				recipeNode = n
				return
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if recipeNode != nil {
				return
			}
			findRecipeNode(child)
		}
	}
	findRecipeNode(doc)

	if recipeNode == nil {
		return nil
	}

	recipe := &RecipeData{}
	collectMicrodata(recipeNode, recipe)

	if recipe.Name == "" && len(recipe.Ingredients) == 0 && len(recipe.Instructions) == 0 {
		return nil
	}
	return recipe
}

func collectMicrodata(node *html.Node, recipe *RecipeData) {
	if node == nil || recipe == nil {
		return
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			itemProp := ""
			content := ""
			for _, attr := range n.Attr {
				key := strings.ToLower(attr.Key)
				switch key {
				case "itemprop":
					itemProp = strings.ToLower(strings.TrimSpace(attr.Val))
				case "content":
					content = strings.TrimSpace(attr.Val)
				}
			}

			if itemProp != "" {
				value := content
				if value == "" {
					value = strings.TrimSpace(nodeText(n))
				}
				switch itemProp {
				case "name":
					if recipe.Name == "" {
						recipe.Name = value
					}
				case "description":
					if recipe.Description == "" {
						recipe.Description = value
					}
				case "image":
					if recipe.Image == "" {
						recipe.Image = value
					}
				case "recipeingredient", "ingredients":
					recipe.Ingredients = appendUnique(recipe.Ingredients, splitAndCleanLines(value)...)
				case "recipeinstructions":
					recipe.Instructions = appendUnique(recipe.Instructions, splitAndCleanLines(value)...)
				case "preptime":
					recipe.PrepTime = FormatDuration(value)
				case "cooktime":
					recipe.CookTime = FormatDuration(value)
				case "totaltime":
					recipe.TotalTime = FormatDuration(value)
				case "recipeyield":
					recipe.Yield = value
				case "recipecuisine":
					recipe.Cuisine = value
				case "recipecategory":
					recipe.Category = value
				case "datepublished":
					recipe.DatePublished = value
				case "author":
					if recipe.Author == "" {
						recipe.Author = value
					}
				case "calories":
					recipe.NutritionInfo = ensureNutrition(recipe.NutritionInfo)
					recipe.NutritionInfo.Calories = value
				case "servings", "servingsize":
					recipe.NutritionInfo = ensureNutrition(recipe.NutritionInfo)
					recipe.NutritionInfo.Servings = value
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
}

func extractRecipeFromHeuristics(doc *html.Node, body []byte) *RecipeData {
	if doc == nil {
		return nil
	}

	recipe := &RecipeData{}
	metaTags, title := extractHTMLMeta(body)
	if title == "" {
		title = metaTags["og:title"]
	}
	recipe.Name = strings.TrimSpace(title)
	recipe.Description = strings.TrimSpace(firstNonEmpty(metaTags["og:description"], metaTags["description"]))
	recipe.Image = strings.TrimSpace(firstNonEmpty(metaTags["og:image"], metaTags["twitter:image"], metaTags["twitter:image:src"]))
	recipe.Author = strings.TrimSpace(firstNonEmpty(metaTags["author"], metaTags["twitter:creator"]))

	recipe.Ingredients = appendUnique(recipe.Ingredients, extractListByClassOrID(doc, "ingredient")...)
	recipe.Ingredients = appendUnique(recipe.Ingredients, extractListByClassOrID(doc, "ingredients")...)
	recipe.Instructions = appendUnique(recipe.Instructions, extractListByClassOrID(doc, "instruction")...)
	recipe.Instructions = appendUnique(recipe.Instructions, extractListByClassOrID(doc, "instructions")...)
	recipe.Instructions = appendUnique(recipe.Instructions, extractListByClassOrID(doc, "direction")...)
	recipe.Instructions = appendUnique(recipe.Instructions, extractListByClassOrID(doc, "directions")...)
	recipe.Instructions = appendUnique(recipe.Instructions, extractListByClassOrID(doc, "method")...)
	recipe.Instructions = appendUnique(recipe.Instructions, extractListByClassOrID(doc, "steps")...)

	if recipe.Name == "" && len(recipe.Ingredients) == 0 && len(recipe.Instructions) == 0 {
		return nil
	}

	return recipe
}

func extractListByClassOrID(doc *html.Node, needle string) []string {
	needle = strings.ToLower(needle)
	if doc == nil || needle == "" {
		return nil
	}
	values := []string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			match := false
			for _, attr := range n.Attr {
				key := strings.ToLower(attr.Key)
				if key != "class" && key != "id" {
					continue
				}
				if strings.Contains(strings.ToLower(attr.Val), needle) {
					match = true
					break
				}
			}
			if match {
				list := extractListItems(n)
				if len(list) > 0 {
					values = append(values, list...)
				} else {
					text := strings.TrimSpace(nodeText(n))
					values = append(values, splitAndCleanLines(text)...)
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return uniqueStrings(values)
}

func extractListItems(node *html.Node) []string {
	if node == nil {
		return nil
	}
	items := []string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "li") {
			text := strings.TrimSpace(nodeText(n))
			if text != "" {
				items = append(items, text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return items
}

func nodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			builder.WriteString(n.Data)
			builder.WriteString(" ")
			return
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "noscript":
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.TrimSpace(builder.String())
}

func parseString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case map[string]interface{}:
		if name := parseString(v["name"]); name != "" {
			return name
		}
		if text := parseString(v["text"]); text != "" {
			return text
		}
		if url := parseString(v["url"]); url != "" {
			return url
		}
	case []interface{}:
		for _, item := range v {
			if parsed := parseString(item); parsed != "" {
				return parsed
			}
		}
	}
	return ""
}

func parseImage(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		if url := parseString(v["url"]); url != "" {
			return url
		}
		if contentURL := parseString(v["contentUrl"]); contentURL != "" {
			return contentURL
		}
		if id := parseString(v["@id"]); id != "" {
			return id
		}
	case []interface{}:
		for _, item := range v {
			if url := parseImage(item); url != "" {
				return url
			}
		}
	}
	return ""
}

func parseStringSlice(values ...interface{}) []string {
	for _, value := range values {
		switch v := value.(type) {
		case []interface{}:
			items := []string{}
			for _, item := range v {
				if parsed := parseString(item); parsed != "" {
					items = append(items, parsed)
				}
			}
			if len(items) > 0 {
				return uniqueStrings(items)
			}
		case []string:
			if len(v) > 0 {
				return uniqueStrings(v)
			}
		case string:
			items := splitAndCleanLines(v)
			if len(items) > 0 {
				return uniqueStrings(items)
			}
		}
	}
	return nil
}

func parseInstructions(value interface{}) []string {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		return splitAndCleanLines(v)
	case []interface{}:
		steps := []string{}
		for _, item := range v {
			steps = append(steps, parseInstructions(item)...)
		}
		return uniqueStrings(steps)
	case map[string]interface{}:
		if text := parseString(v["text"]); text != "" {
			return splitAndCleanLines(text)
		}
		if name := parseString(v["name"]); name != "" {
			return splitAndCleanLines(name)
		}
		if items, ok := v["itemListElement"]; ok {
			return parseInstructions(items)
		}
	}
	return nil
}

func parseAuthor(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		if name := parseString(v["name"]); name != "" {
			return name
		}
		if author := parseString(v["author"]); author != "" {
			return author
		}
	case []interface{}:
		for _, item := range v {
			if author := parseAuthor(item); author != "" {
				return author
			}
		}
	}
	return ""
}

func parseNutrition(value interface{}) *NutritionInfo {
	if value == nil {
		return nil
	}
	nutrition := &NutritionInfo{}
	switch v := value.(type) {
	case map[string]interface{}:
		nutrition.Calories = parseString(v["calories"])
		nutrition.Servings = firstNonEmpty(parseString(v["servingSize"]), parseString(v["servings"]), parseString(v["serving"]))
	case string:
		nutrition.Calories = strings.TrimSpace(v)
	}
	if nutrition.Calories == "" && nutrition.Servings == "" {
		return nil
	}
	return nutrition
}

func ensureNutrition(info *NutritionInfo) *NutritionInfo {
	if info == nil {
		return &NutritionInfo{}
	}
	return info
}

func isRecipeType(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return isRecipeTypeString(v)
	case []interface{}:
		for _, item := range v {
			if isRecipeType(item) {
				return true
			}
		}
	}
	return false
}

func isRecipeTypeString(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	prefixes := []string{
		"http://schema.org/",
		"https://schema.org/",
		"https://schema.org",
		"schema:",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
			break
		}
	}
	return normalized == "recipe"
}

func splitAndCleanLines(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	lines := strings.FieldsFunc(trimmed, func(r rune) bool {
		switch r {
		case '\n', '\r':
			return true
		}
		return false
	})
	if len(lines) == 0 {
		return nil
	}
	results := make([]string, 0, len(lines))
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if clean == "" {
			continue
		}
		results = append(results, clean)
	}
	return results
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	unique := []string{}
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		unique = append(unique, clean)
	}
	return unique
}

func appendUnique(existing []string, values ...string) []string {
	combined := append(existing, values...)
	return uniqueStrings(combined)
}
