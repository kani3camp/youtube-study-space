# Style Comparison Benchmark

Direction A / B / C / D の描画差だけを比較するための再現可能な検証プロトコル。

これは管理人のアートディレクションそのものではなく、**比較のためだけの固定fixture**である。ここに含まれる屋上、テラス、樹木などを各Directionの必須モチーフへ昇格させない。

## Primary protocol: text-locked style fidelity

**Style fidelity の判定は text-only を主とする。**

実生成では、画像 reference を強く固定すると geometry は揃いやすい一方、reference 側の3D形状・陰影・材質表現まで引き継がれ、特に Direction B / C / D の描画言語を弱めることがある。そのため「同じ geometry に見えること」を優先して本来の画風を潰さない。

A/B/C/Dで以下を**完全に同じテキスト**として指定する。

- production の共通用途制約
- 題材
- camera / field of view
- seat count
- seat layout
- major architecture anchors
- time of day
- weather
- light direction
- people / text / UI constraints

変更してよいのは各Directionの `## Prompt guidance` だけ。

生成後に geometry が多少変わった場合は、それを Style fidelity と混ぜず `geometry drift` として切り分けて記録する。

## Secondary protocol: neutral geometry anchor

geometry drift を切り分けたい場合だけ、A/B/C/D のどれにも属さない**スタイル中立の clay / blockout / massing model** を共通 reference として使う。

完成した Direction A/B/C/D 画像を基準にしない。完成絵を基準にすると、その画像固有の材質・光・線・色面まで他の Direction へ伝播してしまう。

neutral anchor は白〜薄いグレー中心にし、次を避ける。

- realistic / PBR material
- cinematic CG lighting
- anime linework
- pastel color design
- painterly texture

### Reference で固定するもの

neutral anchor は**geometryだけ**を固定する。

- camera position and lens / field of view
- architecture and floor plan
- platform heights and stair positions
- seat count and seat positions
- hero structure position and silhouette
- major architectural blocks
- distant skyline / horizon

### Text fixture で固定するもの

condition と production constraints は reference から引き継がず、各 variant で**同じテキストを改めて指定**する。

- time of day
- weather
- light direction
- production の共通用途制約
- people / text / UI constraints
- fixed text fixture の構造条件

中立アンカー自体は評価対象にせず、その材質・陰影・色も引き継がない。

各 variant には上記の text fixture を改めて指定したうえで、次を追加する。

> Preserve only the camera, composition, architecture, floor plan, platform heights, stairs, seating positions, hero structure, major blocks and skyline from the neutral reference image. Do not inherit its materials, shading, colors, time of day, weather or lighting. Use the text fixture for all environmental conditions and production constraints. Do not redesign or relocate objects. Change only the visual rendering language required by the selected Direction.

neutral anchor を用いた結果は Style fidelity の主判定には使わず、`geometry drift` の診断用とする。reference によって Style fidelity が明らかに低下する場合、そのモデルではこの secondary protocol を採用せず、geometry fidelity と style fidelity のトレードオフとして記録する。

## Fixed text fixture

text-only比較では、少なくとも次の構造アンカーをA/B/C/Dで完全に同一にする。

- 16:9
- clear bright early afternoon
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

## Model evaluation

Direction定義は特定モデルへ最適化しすぎない。モデル比較では最低限次を分けて見る。

### Style fidelity
- A: 立体・素材・光・ゲーム環境としての高揚感
- B: 2Dアニメの線・value grouping・色面・空気感
- C: 非写実的な形・色面・余白・簡略化
- D: 整理された具体形・柔らかな陰影・簡略化した素材・クリーンなデジタル仕上げ

### Geometry drift
- カメラが変わっていないか
- 座席位置が変わっていないか
- 主役構造物の位置・形が変わっていないか
- 大きな動線や段差が変わっていないか

### Condition fidelity
- 指定した時間帯を別の時間帯へ変えていないか
- 天気を変えていないか
- 光源方向を大きく変えていないか

### Production usability
- 座席カードを置ける面が十分か
- UI安全領域の可読性があるか
- それ以外の領域が無難に平坦化されていないか

## Subject Swap

比較fixtureで成功しても、題材依存を避けるため別題材で再確認する。

少なくとも1つ、屋上・庭園・青空から大きく離れた題材を使う。例:

- underground research habitat
- enclosed orbital station
- snowfield base
- indoor industrial atrium

Subject Swap では geometry の一致ではなく、各 Direction の描画原理が残ることを優先する。

