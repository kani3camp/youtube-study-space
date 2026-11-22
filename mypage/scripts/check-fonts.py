"""MyPageに同梱しているフォントファイルが記録どおりの状態であり、破損や改変がないかをオフラインで検査する。

目的: フォントの差し替えや加工ミスで、ファイルの中身・文字数・太さの指定が登録した内容と食い違う事故を防ぐ。
使われる場面: CI Frontend ワークフロー(.github/workflows/ci-frontend.yml)の mypage-check ジョブの「Validate local font rendering and integrity」ステップ。
失敗したとき: CI Frontend が失敗し、PRをマージできない。
なくした場合: 同梱フォントの改変や破損が自動では見つからなくなる。ブラウザで実際の表示を見る font-qa.py とは別の検査(こちらはファイル自体の検査)。

補足: 実行の前に font-requirements.txt を導入する(fontTools が必要)。
"""

import hashlib
import json
from pathlib import Path

from fontTools.ttLib import TTFont


ROOT = Path(__file__).resolve().parents[1]


def preserved_tables_sha256(font):
    """gasp以外の全テーブルを含め、head.checkSumAdjustmentだけを正規化する。"""
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
            # 意味を解釈するアクセスで遅延再コンパイルが始まる前に、元のテーブルを読む。
            assert preserved_tables_sha256(font) == face["preservedTablesSHA256"], path.name
            assert font.flavor == "woff2", path.name
            assert font["gasp"].version == 1, path.name
            assert font["gasp"].gaspRange == {65535: 0x000A}, path.name
            assert len(font.getGlyphOrder()) == face["glyphs"], path.name
            assert font["OS/2"].usWeightClass == face["weight"], path.name
        print(f"{path.name}: smoothing v1/0x000A; original tables preserved")


if __name__ == "__main__":
    check_fonts()
