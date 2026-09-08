# Live Room Scenes rollout runbook

This runbook covers preview, OBS acceptance, production enablement, monitoring, and rollback for Live Room Scenes.

Technical architecture: [Live Room Scenes technical design](./live-room-scenes.md)

## Safety model

Static rendering is the authoritative fallback.

A room may contain a `scene` config while production remains completely Static. Ambient/Living rendering requires an explicit build-time opt-in:

```text
NEXT_PUBLIC_LIVE_ROOM_SCENES_ENABLED=true
```

If the variable is missing or not exactly `true`, Live Room Scenes stay disabled.

The runtime emergency kill switch is:

```text
?liveScenes=off
```

It overrides an enabled build without requiring a rebuild.

## Phase 0: Static baseline

Before enabling scenes, verify the same build with:

```text
NEXT_PUBLIC_LIVE_ROOM_SCENES_ENABLED=false
```

or with the variable omitted.

Acceptance:

- room paging behaves exactly as before;
- seat count/order and max-seat behavior are unchanged;
- `CafeRainyRoom` renders its existing `floor_image`;
- no Living canvas is mounted;
- the root element exposes `data-live-room-scenes="off"`.

Do not proceed if the Static baseline differs from the current production monitor.

## Phase 1: compressed visual preview

Use a non-production DEBUG build. DEBUG changes other monitor behavior, so do not use this URL as the public production source.

Build with:

```text
NEXT_PUBLIC_DEBUG=true
NEXT_PUBLIC_LIVE_ROOM_SCENES_ENABLED=false
```

Then open:

```text
?liveScenes=on&sceneTime=00:00&sceneSpeed=720
```

This compresses 24 scene hours into two real minutes.

Review at minimum:

- dawn transition around 04:55 JST;
- day around 09:15 JST;
- sunset around 17:40 JST;
- night around 19:55 JST;
- midnight around 00:25 JST.

For Lume / Main Café Floor / Rainy, verify:

- rain stays visually sparse and slow;
- rain remains inside the intended window area;
- sunset/night grading does not obscure furniture or room identity;
- lamp glow strengthens gradually rather than flashing;
- seat boxes, names, work labels, and partitions remain above the scene;
- there is no camera or furniture motion.

Also check the exact anchor states with frozen time, for example:

```text
?liveScenes=on&sceneTime=09:15
?liveScenes=on&sceneTime=17:40
?liveScenes=on&sceneTime=19:55
?liveScenes=on&sceneTime=00:25
```

## Phase 2: OBS smoke test

Use the same browser-source dimensions and output path as production:

- browser source: 1920x1080;
- target scene animation: max 30fps;
- normal production room paging;
- existing stream encoder settings unchanged.

Run both states against the same machine/session:

1. Static baseline with `?liveScenes=off`.
2. Living enabled with the query override removed.

Observe:

- OBS rendering lag;
- OBS encoding lag;
- dropped frames;
- browser/CEF process CPU and GPU usage;
- browser/CEF memory;
- room-switch latency;
- visible tearing, blank frames, or canvas flashes.

Acceptance is comparative, not an invented absolute GPU percentage: enabling scenes must not introduce sustained render/encode lag, a new dropped-frame pattern, or a steadily growing browser-memory trend relative to the Static baseline.

Run the smoke comparison for at least 30 minutes and include repeated page switches through Lume.

## Phase 3: recovery checks

Before production enablement, prove each fallback path.

### Runtime kill switch

While scenes are enabled, append:

```text
?liveScenes=off
```

Refresh the OBS browser source.

Expected:

- Lume immediately returns to the original Static image;
- seat/UI behavior is unchanged;
- no rebuild is required.

Remove the query parameter and refresh to re-enable only after the issue is understood.

### WebGL context loss

Where browser developer tooling permits safe context-loss simulation:

1. Keep Lume active.
2. Trigger WebGL context loss.
3. Confirm the Living layer disappears and Static remains.
4. Restore the context.
5. Confirm Living resumes only if Lume is still the active attached room.

Repeat while switching away from Lume before context restoration.

Expected: the old room must not restart after the delayed restore.

### Initialization failure

Use a test/debug environment where WebGL/Pixi initialization can be made unavailable.

Expected:

- Static image remains visible;
- page switching continues;
- a later activation can retry initialization.

## Phase 4: soak test

Before treating Live Room Scenes as production-stable:

1. Run at least a two-hour non-public OBS soak with normal room paging.
2. Record browser/CEF memory near the start and periodically during the run.
3. Confirm memory reaches a stable range rather than increasing with every Lume activation.
4. Confirm no accumulation of canvases or additional WebGL contexts.
5. Confirm no render/encode lag appears after repeated activation/deactivation.

After initial production enablement, keep the first 24 hours as a monitored canary period. The feature should not be considered fully rolled out until that period completes without a scene-related rollback.

## Production enablement

Only after the preceding checks pass:

1. Build/deploy with:

   ```text
   NEXT_PUBLIC_LIVE_ROOM_SCENES_ENABLED=true
   ```

2. Keep the normal production URL without a `liveScenes` force-on parameter.
3. Confirm the root element exposes `data-live-room-scenes="on"`.
4. Observe the first Lume activation and at least one room switch away/back.
5. Keep `?liveScenes=off` ready as the first rollback action.

Because `NEXT_PUBLIC_*` configuration is bundled into client code, changing the environment variable requires rebuilding/redeploying the monitor. The URL kill switch does not.

## Rollback

Use the smallest rollback that restores a stable stream.

### Level 1: immediate runtime rollback

Append `?liveScenes=off` to the OBS browser-source URL and refresh the source.

Use this first for:

- unexpected visual artifacts;
- GPU/CPU pressure;
- rendering/encoding lag;
- WebGL instability;
- suspected memory growth.

### Level 2: persistent configuration rollback

Set:

```text
NEXT_PUBLIC_LIVE_ROOM_SCENES_ENABLED=false
```

then rebuild/redeploy.

This restores Static rendering without reverting the feature code.

### Level 3: code rollback

Only if Static behavior is affected despite the feature switch being off, revert the Live Room Scenes integration. This would violate the project invariant and should be treated as a release-blocking defect.

## Go / no-go checklist

Code/CI:

- [ ] Biome passes
- [ ] Jest passes
- [ ] Next production build passes
- [ ] Static fallback tests pass
- [ ] rollout switch tests pass
- [ ] WebGL recovery tests pass

Visual:

- [ ] compressed 24h Lume preview reviewed
- [ ] anchor times reviewed
- [ ] seat/name/work readability preserved
- [ ] motion remains calm enough for a work stream

OBS:

- [ ] 30-minute Static vs Living smoke comparison passes
- [ ] runtime kill switch verified
- [ ] context-loss fallback verified where tooling allows
- [ ] two-hour soak passes
- [ ] no monotonic memory growth observed

Production:

- [ ] explicit enable flag set only after acceptance
- [ ] first activation monitored
- [ ] first 24 hours treated as canary
- [ ] rollback URL documented for the operator
