from PIL import Image
from pathlib import Path
import sys


def main():
    if len(sys.argv) < 2:
        print("Usage: python gen_favicon_assets.py <output_dir>")
        sys.exit(1)

    web_png = Path("branding/icon/web_png")
    output_dir = Path(sys.argv[1])
    output_dir.mkdir(parents=True, exist_ok=True)

    img_64 = Image.open(web_png / "64px.png").convert("RGBA")
    img_32 = Image.open(web_png / "32px.png").convert("RGBA")
    img_16 = Image.open(web_png / "16px.png").convert("RGBA")

    ico_path = output_dir / "favicon.ico"
    img_32.save(ico_path, format="ICO", sizes=[(16, 16), (32, 32)])
    print(f"favicon.ico (16+32) → {ico_path}")

    sizes = {
        "apple-touch-icon.png": 180,
        "android-chrome-192x192.png": 192,
        "android-chrome-512x512.png": 512,
    }

    for filename, size in sizes.items():
        out = img_64.resize((size, size), Image.NEAREST)
        out_path = output_dir / filename
        out.save(out_path)
        print(f"{filename} ({size}x{size}) → {out_path}")


if __name__ == "__main__":
    main()
