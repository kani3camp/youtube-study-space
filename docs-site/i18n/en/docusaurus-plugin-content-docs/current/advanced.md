---
title: Advanced Commands
sidebar_position: 3
---

## !rank

### Toggle rank display mode

```
!rank
```

Toggle rank display mode on/off.

## !my

You can set your favorite settings for each user.
Currently supported settings are:

- Rank display mode on/off
- Change/reset your favorite work time (default work time)
- Set/reset your favorite color

You need to specify at least one option.

### Set rank display mode on/off

#### Turn rank display mode on.

```
!my rank on
```

#### Turn rank display mode off.

```
!my rank off
```

### Set favorite work time

You can specify your favorite work time (default work time) in minutes with the `min` option.

With this setting, when you use the [`!in` command](/docs/essential#in), you will automatically enter with the [`min` option](/docs/essential#min-option) applied.

```text title="Example: Set favorite work time to 60 minutes."
!my min 60
```

```text title="Example: Reset favorite work time."
!my min
```

### Set favorite color

You can specify your favorite color with the `color` option.
Available color names can be found [here](https://youtube-study-space.notion.site/f4366038a5de4fe1957bfbfa93fd1ebb?v=4dcfe9a135d54615a84083b9dd3d7f5f).

You can set your [favorite color](https://youtube-study-space.notion.site/3fc22ea1b4214b3f976b03331c51d113).

```text title="Example: Set favorite color to アクアマリン."
!my color アクアマリン
```

```text title="Example: Reset favorite color."
!my color
```

## !break

### Take a break

```
!break
```

Enter break mode while in the room.
This cannot be used if you're not in the room.

:::warning
You cannot enter break mode during the first few minutes after entering the room or during the first few minutes after ending a break.
:::

:::warning
Break mode time is not added to your total work time.
:::

:::info
The Pomodoro timer on the screen may also display "Break", but this is not related to break mode. It is also not related to your total work time.
:::

:::info
`!rest` and `!chill` can also be used as commands with the same meaning as `!break`. Use whichever you prefer.
:::

### Additional options `work` `min`

You can specify break content with the `work` option.
You can also specify break time in minutes with the `min` option.

The break content will be deleted when the break ends, and your original work name will be restored.

```text title="Example: Take a 30-minute break with the break content set to 'Short break'."
!break work Short break min 30
```

```text title="Example: Take a 20-minute break. After 20 minutes, break mode will automatically end and work will resume."
!break min 20
```

## !resume

### Resume work

```
!resume
```

Resume work from break mode.
This can only be used when in break mode.

Break mode will automatically end after the specified time, but if you want to return to work earlier, you can use this command to immediately resume work.
