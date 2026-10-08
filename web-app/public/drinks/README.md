# Drink illustrations

Line-art drink images for the kiosk menu cards and recipe pages, in the same style as the prototype.

| File slug | Drink |
|---|---|
| `espresso-martini` | Espresso martini (coupe, foam, coffee beans) |
| `martini` | Martini (V glass, olive) |
| `negroni` | Negroni (rocks glass, ice, orange peel) |
| `old-fashioned` | Old fashioned (rocks glass, big cube, orange peel, cherry) |
| `margarita` | Margarita (coupe, salt rim, lime wheel) |
| `aperol-spritz` | Aperol spritz (wine glass, bubbles, orange slice) |
| `moscow-mule` | Moscow mule (copper mug, lime, mint) |
| `gin-and-tonic` | Gin and tonic (highball, bubbles, lime) |
| `mocktail` | Mocktail (highball, berries, mint, straw) |

## Formats

- `svg/` — vector, 240 × 240 viewBox. Outlines use `currentColor`, so when the SVG is **inlined** the outline follows the text color and works in light and dark mode automatically. (Through an `<img>` tag, `currentColor` falls back to black; use the PNGs there.)
- `png/light/` — 600 × 600, transparent background, dark outlines (`#1D1D1F`). For light backgrounds.
- `png/dark/` — 600 × 600, transparent background, light outlines (`#F5F5F7`). For dark backgrounds.

## Using them in the web app

1. Copy this folder to `web-app/public/drinks/`.
2. Give each recipe an `image` field with the slug (e.g. `"image": "espresso-martini"`). Recipes without one fall back to `mocktail` or a plain glass.
3. On the kiosk card, either inline the SVG, or use a `<picture>` so the right PNG shows in each mode:

```html
<picture>
  <source srcset="/drinks/png/dark/espresso-martini.png" media="(prefers-color-scheme: dark)">
  <img src="/drinks/png/light/espresso-martini.png" alt="" width="150" height="150">
</picture>
```

The images are decorative (the drink name is right under them), so `alt=""` is correct.
