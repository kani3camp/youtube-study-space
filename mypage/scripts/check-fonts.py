"""Offline font integrity/rendering check; install font-requirements.txt first."""

import hashlib
import json
from pathlib import Path

from fontTools.ttLib import TTFont


ROOT = Path(__file__).resolve().parents[1]


def preserved_tables_sha256(font):
    """Include all tables except gasp; normalize only head.checkSumAdjustment."""
    digest = hashlib.sha256()
    for tag in sorted(set(font.keys()) - {"GlyphOrder", "gasp"}):
        data = font.getTableData(tag)
        if tag == "head":
            data = data[:8] + bytes(4) + data[12:]
        digest.update(tag.encode("ascii"))
        digest.update(len(data).to_bytes(4, "big"))
        digest.update(data)
    return digest.hexdigest()


def check_fonts():
    sources = json.loads((ROOT / "src/assets/fonts/sources.json").read_text())
    assert [face["weight"] for face in sources["faces"]] == [400, 500, 700, 800]
    for face in sources["faces"]:
        path = ROOT / "src/assets/fonts" / face["woff2File"]
        data = path.read_bytes()
        assert len(data) == face["woff2Bytes"], path.name
        assert hashlib.sha256(data).hexdigest() == face["woff2SHA256"], path.name
        with TTFont(path, recalcTimestamp=False) as font:
            # Read raw tables before semantic access can trigger lazy recompile.
            assert preserved_tables_sha256(font) == face["preservedTablesSHA256"], path.name
            assert font.flavor == "woff2", path.name
            assert font["gasp"].version == 1, path.name
            assert font["gasp"].gaspRange == {65535: 0x000A}, path.name
            assert len(font.getGlyphOrder()) == face["glyphs"], path.name
            assert font["OS/2"].usWeightClass == face["weight"], path.name
        print(f"{path.name}: smoothing v1/0x000A; original tables preserved")


if __name__ == "__main__":
    check_fonts()
