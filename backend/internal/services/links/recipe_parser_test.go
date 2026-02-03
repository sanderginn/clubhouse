package links

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestParseRecipeSchemaJSONLD(t *testing.T) {
	htmlBody := `<!doctype html>
<html>
<head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Recipe",
  "name": "Best Pancakes",
  "description": "Fluffy pancakes",
  "image": "https://example.com/pancakes.jpg",
  "recipeIngredient": ["1 cup flour", "1 egg"],
  "recipeInstructions": [
    {"@type": "HowToStep", "text": "Mix ingredients"},
    {"@type": "HowToStep", "text": "Cook on griddle"}
  ],
  "prepTime": "PT10M",
  "cookTime": "PT5M",
  "totalTime": "PT15M",
  "recipeYield": "4 servings",
  "author": {"@type": "Person", "name": "Chef A"},
  "datePublished": "2024-01-01",
  "recipeCuisine": "American",
  "recipeCategory": "Breakfast",
  "nutrition": {"calories": "200 calories", "servingSize": "1 serving"}
}
</script>
</head>
</html>`

	recipe, err := ParseRecipeSchema([]byte(htmlBody))
	if err != nil {
		t.Fatalf("ParseRecipeSchema error: %v", err)
	}
	if recipe == nil {
		t.Fatal("expected recipe data")
	}
	if recipe.Name != "Best Pancakes" {
		t.Errorf("Name = %q, want %q", recipe.Name, "Best Pancakes")
	}
	if len(recipe.Ingredients) != 2 {
		t.Fatalf("Ingredients len = %d, want 2", len(recipe.Ingredients))
	}
	if len(recipe.Instructions) != 2 {
		t.Fatalf("Instructions len = %d, want 2", len(recipe.Instructions))
	}
	if recipe.PrepTime != "10m" {
		t.Errorf("PrepTime = %q, want %q", recipe.PrepTime, "10m")
	}
	if recipe.CookTime != "5m" {
		t.Errorf("CookTime = %q, want %q", recipe.CookTime, "5m")
	}
	if recipe.TotalTime != "15m" {
		t.Errorf("TotalTime = %q, want %q", recipe.TotalTime, "15m")
	}
	if recipe.NutritionInfo == nil || recipe.NutritionInfo.Calories != "200 calories" {
		t.Errorf("Nutrition calories = %v, want %q", recipe.NutritionInfo, "200 calories")
	}
}

func TestExtractRecipeFromHTMLMicrodata(t *testing.T) {
	htmlBody := `<!doctype html>
<html>
<body>
<div itemscope itemtype="https://schema.org/Recipe">
  <h1 itemprop="name">Quick Salad</h1>
  <ul>
    <li itemprop="recipeIngredient">1 tomato</li>
    <li itemprop="recipeIngredient">2 cucumbers</li>
  </ul>
  <div itemprop="recipeInstructions">Chop and mix.</div>
  <meta itemprop="prepTime" content="PT5M" />
</div>
</body>
</html>`

	recipe, err := ExtractRecipeFromHTML([]byte(htmlBody), "example.com")
	if err != nil {
		t.Fatalf("ExtractRecipeFromHTML error: %v", err)
	}
	if recipe == nil {
		t.Fatal("expected recipe data")
	}
	if recipe.Name != "Quick Salad" {
		t.Errorf("Name = %q, want %q", recipe.Name, "Quick Salad")
	}
	if len(recipe.Ingredients) != 2 {
		t.Fatalf("Ingredients len = %d, want 2", len(recipe.Ingredients))
	}
	if len(recipe.Instructions) != 1 {
		t.Fatalf("Instructions len = %d, want 1", len(recipe.Instructions))
	}
	if recipe.PrepTime != "5m" {
		t.Errorf("PrepTime = %q, want %q", recipe.PrepTime, "5m")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "PT30M", want: "30m"},
		{input: "PT1H15M", want: "1h 15m"},
		{input: "P1DT2H", want: "1d 2h"},
		{input: "nope", want: "nope"},
	}

	for _, tt := range tests {
		if got := FormatDuration(tt.input); got != tt.want {
			t.Errorf("FormatDuration(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFetchMetadataIncludesRecipe(t *testing.T) {
	htmlBody := `<!doctype html>
<html>
<head>
  <meta property="og:title" content="Recipe Page" />
  <script type="application/ld+json">{"@context":"https://schema.org","@type":"Recipe","name":"Recipe Page","recipeIngredient":["1 cup water"]}</script>
</head>
</html>`

	fetcher := NewFetcher(&http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(htmlBody)),
				Request:    r,
			}, nil
		}),
	})
	fetcher.resolver = fakeResolver{
		addrs: map[string][]net.IPAddr{
			"example.com": {{IP: net.ParseIP("93.184.216.34")}},
		},
	}

	metadata, err := fetcher.Fetch(context.Background(), "https://example.com/recipe")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}

	recipeValue, ok := metadata["recipe"]
	if !ok {
		t.Fatalf("expected recipe metadata")
	}
	recipe, ok := recipeValue.(*RecipeData)
	if !ok || recipe == nil {
		t.Fatalf("recipe metadata has unexpected type %T", recipeValue)
	}
	if recipe.Name != "Recipe Page" {
		t.Errorf("recipe.Name = %q, want %q", recipe.Name, "Recipe Page")
	}
}
