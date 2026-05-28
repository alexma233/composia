from PIL import Image
from collections import defaultdict
import sys

if len(sys.argv) < 3:
    print("Usage: python pixel_to_svg_path.py input.png output.svg")
    sys.exit(1)

input_path = sys.argv[1]
output_path = sys.argv[2]

img = Image.open(input_path).convert("RGBA")
w, h = img.size
pixels = img.load()

paths = defaultdict(list)

for y in range(h):
    x = 0
    while x < w:
        r, g, b, a = pixels[x, y]

        if a == 0:
            x += 1
            continue

        start_x = x
        color = (r, g, b, a)

        while x < w and pixels[x, y] == color:
            x += 1

        run_width = x - start_x

        if a == 255:
            key = f'fill="#{r:02x}{g:02x}{b:02x}"'
        else:
            opacity = round(a / 255, 4)
            key = f'fill="#{r:02x}{g:02x}{b:02x}" opacity="{opacity}"'

        paths[key].append(f"M{start_x} {y}h{run_width}v1h-{run_width}z")

svg = [
    f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" shape-rendering="crispEdges">'
]

for attrs, d_parts in paths.items():
    svg.append(f'<path {attrs} d="{" ".join(d_parts)}"/>')

svg.append("</svg>")

with open(output_path, "w", encoding="utf-8") as f:
    f.write("\n".join(svg))
