# Style Comparison Benchmark

Direction A / B / C の描画差だけを比較するための再現可能な検証プロトコル。

これは管理人のアートディレクションそのものではなく、**比較のためだけの固定fixture**である。ここに含まれる屋上、テラス、樹木などを各Directionの必須モチーフへ昇格させない。

## Preferred protocol: image-reference lock

画像参照を使えるモデルでは、まず1枚の基準構図を作る。その画像をA/B/Cすべてへ同じ `image` reference として渡し、以下を固定する。

- camera position and lens / field of view
- architecture and floor plan
- platform heights and stair positions
- seat count and seat positions
- hero structure position and silhouette
- distant skyline / horizon
- time of day
- weather
- light direction
- major hue placement when possible

各variantには次を追加する。

> Preserve the exact camera, composition, architecture, floor plan, platform heights, stairs, seating positions, hero structure, skyline, time of day, weather and lighting direction from the reference image. Do not redesign or relocate objects. Change only the visual rendering language required by the selected Direction.

比較時は、スタイル差と同時に構図が変わった場合、その差を `geometry drift` として別評価する。

## Text-only fallback fixture

画像参照を使えない場合は、少なくとも次の構造アンカーをA/B/Cで完全に同一にする。

- 16:9
- clear early afternoon
- dry weather with a few fair-weather clouds
- sunlight from upper left
- elevated three-quarter bird's-eye camera
- 12 seats
- one main stepped terrace in the center
- six work positions distributed across the central terrace
- three work positions along a left-side raised band
- three work positions along a right-side raised band
- one dominant pedestrian route crossing front-left to back-right
- one large hero structure in the upper-left quadrant
- one open scenic vista in the upper-right quadrant
- the same plant masses and major architectural blocks in the same locations

Directionごとに変更してよいのは、線、色面、シェーディング、素材表現、光の描き方、ディテール抽象度などの**描画言語**だけ。

## Review axes

### Style fidelity
- A: 立体・素材・光・ゲーム環境としての高揚感
- B: 2Dアニメの線・value grouping・色面・空気感
- C: 非写実的な形・色面・余白・簡略化

### Geometry drift
- カメラが変わっていないか
- 座席位置が変わっていないか
- 主役構造物の位置・形が変わっていないか
- 大きな動線や段差が変わっていないか

### Production usability
- 座席カードを置ける面が十分か
- UI安全領域の可読性があるか
- それ以外の領域が無難に平坦化されていないか
