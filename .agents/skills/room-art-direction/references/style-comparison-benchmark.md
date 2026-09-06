# Style Comparison Benchmark

Direction A / B / C の描画差だけを比較するための再現可能な検証プロトコル。

これは管理人のアートディレクションそのものではなく、**比較のためだけの固定fixture**である。ここに含まれる屋上、テラス、樹木などを各Directionの必須モチーフへ昇格させない。

## Primary protocol: text-locked style fidelity

**Style fidelity の判定は text-only を主とする。**

実生成では、画像referenceを強くロックするとgeometryは揃いやすい一方、reference側の3D形状・陰影・材質表現まで残り、特にDirection B / Cの描画言語を弱める場合があった。そのため「同じgeometryに見えること」を優先して本来の画風を潰さない。

A/B/Cで以下を**完全に同じテキスト**として指定する。

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

生成後にgeometryが多少変わった場合は、それをStyle fidelityと混ぜず `geometry drift` として別記録する。

## Secondary protocol: neutral geometry anchor

同一geometryでの変換能力も確認したい場合だけ、A/B/Cのどれにも属さない**スタイル中立のclay / blockout / massing model**を共通referenceとして使う。

完成したDirection A/B/C画像を基準にしない。完成絵を基準にすると、その画像固有の材質・光・線・色面まで他Directionへ伝播する。

neutral anchor は白〜薄いグレー中心にし、次を避ける。

- realistic / PBR material
- cinematic CG lighting
- anime linework
- pastel color design
- painterly texture

固定対象:

- camera position and lens / field of view
- architecture and floor plan
- platform heights and stair positions
- seat count and seat positions
- hero structure position and silhouette
- distant skyline / horizon
- time of day
- weather
- light direction

中立アンカー自体は評価対象にしない。目的はgeometry lockだけである。

各variantには production の共通用途制約とbenchmark条件を**再度テキストでも指定**した上で、次を追加する。

> Preserve the exact camera, composition, architecture, floor plan, platform heights, stairs, seating positions, hero structure, skyline, time of day, weather and lighting direction from the neutral reference image. Do not redesign or relocate objects. Change only the visual rendering language required by the selected Direction. Ignore the neutral reference's clay material and placeholder shading.

referenceによってStyle fidelityが明らかに低下する場合、そのモデルではこのsecondary protocolを採用しない。geometry fidelityとstyle fidelityのトレードオフとして記録する。

## Fixed text fixture

text-only比較では、少なくとも次の構造アンカーをA/B/Cで完全に同一にする。

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

Subject Swapではgeometry一致ではなく、各Directionの描画原理が残ることを優先する。


## Secondary diagnostic: neutral geometry anchor

geometry driftを切り分けたい場合だけ、スタイル中立のclay / blockout / massing model画像を補助的に使う。

- 完成したA/B/C画像をreferenceにしない
- neutral anchorは白〜薄いグレー中心で、PBR、アニメ線、パステル色面を持たせない
- productionの共通用途制約とfixed text fixtureを**必ず同時に再宣言**する
- anchorの材質・陰影・色は無視し、camera / floor plan / seat positions / major blocksだけを保持させる

neutral anchor結果はStyle fidelityの主判定には使わず、`geometry drift` の診断用とする。
