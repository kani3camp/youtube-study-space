# Direction D: Clean Cozy Digital Environment Illustration

## Summary

クリーンで親しみやすい、2D寄りのデジタル環境イラスト。写真・建築CGの物理的リアリティではなく、整理された形、滑らかな色面、明快な明暗グループ、調和した配色で空間を描く。

題材やレイアウトではなく「どう描くか」の定義である。具体的な空間として自然に読める立体感は残すが、素材や光を物理的に再現する方向へは寄せない。

## Core

### 形と色面
- 家具、建築、植物、小物、遠景を、読みやすいシルエットと整理された shape / color plane で描く。
- 物体は「物理的にレンダリングされた素材」ではなく、「形としてデザインされた物」として扱う。
- 大きな面を smooth matte color planes として見せ、細部の質感より形と色の関係を優先する。

### 明暗と陰影
- 1つの物体につき、明度はおおむね3〜4段階までに整理する。
- 光、影、中間色を大きくまとめ、小さな陰影の起伏を増やしすぎない。
- soft shading は使ってよいが、airbrush 的な連続グラデーションや写実的な光の減衰で立体を作らない。
- 接地感と奥行きは残しつつ、画面全体は一目でイラストと分かるようにする。

### 素材の簡略化
- 木、布、ガラス、金属、植物は、色・明度・少量のハイライトで判別できればよい。
- 木目は省略するか、ごく少数のグラフィックな線だけにする。
- 布目、石肌、細かな粗さ、鏡面反射などのマイクロテクスチャは描き込まない。
- 雨、水面、濡れた床などの反射は、必要なら少数の単純化された色形として表現する。

### 色と仕上げ
- 暖色と寒色を明快に分け、落ち着きながらも色彩の豊かさを感じる配色にする。
- 茶・ベージュ・オレンジだけへ平均化せず、テーマに合う青、緑、クリーム、アクセント色などを自然に使う。
- エッジは明瞭に保ち、物体同士の境界が溶けないようにする。
- 線画は最小限またはほぼ無しとし、形・色・明度差で物体を分離する。
- blur、haze、bloom、紙・水彩・ブラシの質感で雰囲気を作らない。

## Prefer

- clean 2D digital environment illustration
- smooth matte color planes
- designed readable shapes
- 3〜4 level value grouping
- broad light / midtone / shadow masses
- crisp object separation
- simplified materials
- clear warm / cool color relationships
- visually rich but tidy details
- calm, cozy, approachable atmosphere

## Avoid

- photorealism / architectural visualization / interior CG
- PBR material rendering / ray-traced look
- realistic wood grain, fabric weave, stone roughness, complex reflections
- continuous airbrushed shading
- painterly concept art / watercolor / paper texture
- anime background painting or strong cel-shading
- soft-focus / haze / bloom
- muddy or washed-out low-contrast color
- thick outlines
- excessive micro-detail

## Non-goals

図書館、カフェ、書斎、植物、木製家具、本、ランプ、雨、夜景などは Direction D の必須要素ではない。これらは題材であって画風ではない。

題材、時間帯、天気、建築、レイアウトを変えても、整理された具体形、3〜4段階程度の明暗、簡略化された素材、明快な色面、クリーンなデジタル仕上げが残れば Direction D とみなす。

## Relationship to other directions

- Direction A より素材・反射・物理ライティングを弱め、形と色面の整理を優先する。
- Direction B より線とセル影への依存を弱め、輪郭線の少ないデジタル色面で描く。
- Direction C より具体的な物体形状と空間の立体感を残す。
- Direction D は「具体的な空間として読めるが、レンダーではなく整理された2Dデジタルイラストとして見えること」が判断基準。

## Validation baseline

Direction D の canonical な品質判定は、ChatGPT Chat で GPT-5.6 Sol / High を使った生成結果を主基準とする。

Codex / Work / 軽量モデルでの失敗を補正するためのモデル固有の長いnegativeは canonical prompt へ積み増さない。別経路の生成結果は比較材料として扱い、Direction自体の描画文法を歪めない。

## Prompt guidance

- make the entire rendering unmistakably a clean 2D digital environment illustration, not a physically rendered 3D scene
- build furniture, architecture, plants, props and distant scenery from designed shapes and smooth matte color planes rather than realistic materials
- keep each object to roughly 3〜4 value levels, organizing light, midtone and shadow into broad readable groups instead of many small tonal variations
- use soft controlled shading, but avoid continuous airbrushed gradients; use gentle gradients only sparingly to support form
- keep edges crisp and object separation clear; define forms primarily through shape, color and value rather than heavy outlines
- simplify materials aggressively: suggest wood, fabric, glass, metal and foliage through local color, value and a few controlled highlights; omit micro-texture
- omit wood grain or reduce it to only a few graphic lines; do not render fabric weave, stone roughness or physically accurate reflections
- if rain, wet surfaces or reflected light are present, reduce reflections to a small number of simplified color shapes rather than realistic optical reflections
- simplify plants, books and small props into tidy illustrated forms while keeping enough variation and detail for the room to feel visually rich
- use clear, harmonious color separation with a pleasant warm / cool balance; avoid dull, muddy or washed-out color
- preserve the requested time of day, weather and lighting condition, but stylize them with the same simplified 2D rendering language
- no photorealism, architectural visualization, PBR look, painterly texture, watercolor, paper grain, haze, bloom, soft-focus, thick outlines or strong anime cel-shading

## Review checklist

- [ ] 一目で写真・建築CGではなく2Dデジタルイラストと分かる
- [ ] 家具・建築・植物が「素材」より「形と色面」として読める
- [ ] 1物体あたりの明暗が概ね3〜4段階に整理されている
- [ ] airbrush 的な連続グラデーションや写実的な反射に戻っていない
- [ ] 木・布・ガラスなどが判別できるが、マイクロテクスチャが主役ではない
- [ ] エッジと物体分離が明瞭で、hazeやsoft-focusで溶けていない
- [ ] 暖色・寒色の関係が明快で、色が灰色や茶色へ平均化されていない
- [ ] 小物や植物に適度なバリエーションがあり、単調すぎず過密でもない
- [ ] Direction A/B/C と描画言語が区別できる
- [ ] 特定の家具、植物、窓、時間帯、天気をDirectionの必須条件にしていない
