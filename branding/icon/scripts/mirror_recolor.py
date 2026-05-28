from PIL import Image
from pathlib import Path
import sys


COLOR_MAP = {
    (0x55, 0xAC, 0xEE): (0xF5, 0x9E, 0x0B),  # bright blue       → amber-500
    (0x66, 0x75, 0x7F): (0xD9, 0x77, 0x06),  # blue-gray dark    → amber-600
    (0x99, 0xAA, 0xB5): (0xFB, 0xBF, 0x24),  # blue-gray light   → amber-400
}


def recolor_pixel(r, g, b):
    key = (r, g, b)
    return COLOR_MAP.get(key, key)


def main():
    if len(sys.argv) < 3:
        print("Usage: python mirror_recolor.py <input_dir> <output_dir>")
        sys.exit(1)

    input_dir = Path(sys.argv[1])
    output_dir = Path(sys.argv[2])
    output_dir.mkdir(parents=True, exist_ok=True)

    for png_path in sorted(input_dir.glob("*.png")):
        img = Image.open(png_path).convert("RGBA")
        w, h = img.size

        mirrored = img.transpose(Image.FLIP_LEFT_RIGHT)
        pixels = list(mirrored.get_flattened_data())

        recolored = []
        for r, g, b, a in pixels:
            if a == 0:
                recolored.append((0, 0, 0, 0))
            else:
                nr, ng, nb = recolor_pixel(r, g, b)
                recolored.append((nr, ng, nb, a))

        out = Image.new("RGBA", (w, h))
        out.putdata(recolored)

        out_path = output_dir / png_path.name
        out.save(out_path)
        print(f"{png_path} → {out_path}  ({w}x{h})")


if __name__ == "__main__":
    main()
