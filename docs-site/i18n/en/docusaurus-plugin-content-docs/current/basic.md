---
title: Basic Commands
sidebar_position: 2
---

## !info

### Display work history

```
!info
```

The Bot will tell you your total work time and the current day's total work time.

### Detailed option `d`

```
!info d
```

You can check the following additional information:

- Rank display mode on/off
- (If rank display mode is on) Rank points
- (If rank display mode is on) Consecutive days
- Default work time
- Favorite color
- Registration date for the online work room

## !seat

### Display seat information

```
!seat
```

The Bot will tell you the following information about the seat you're currently sitting at:

- Current seat number
- Time elapsed since entering the room
- Working time (excluding break mode time)
- Remaining time until automatic exit

### Detailed option `d`

```
!seat d
```

Additionally, you can check how much time you've recently spent in your current seat.

## !change

Change your work name or entry time.
This command can only be used while you're in the room.
You need to specify at least one option.

If you're in break mode, you can change the break content and break time.

### Change work name

You can change your work name with the `work=` or `w=` option.

```text title="Example: Change work name to English."
!change work=English
```

```text title="Example: Change work name to Physics."
!change w=Physics
```

```text title="Example: Remove work name."
!change work=
```

### Change entry time

You can change your entry time with the `min=` or `m=` option.

```text title="Example: Change entry time to 40 minutes. If 10 minutes have already passed since entering, the automatic exit time will be set to 30 minutes later (= 40 minutes after entry time)."
!change min=40
```

```text title="Example: Remove work name and change entry time to 5 minutes. For example, if 3 minutes have passed since entry, the automatic exit time will be set to 2 minutes later (= 5 minutes after entry time)."
!change w= m=5
```

:::info
If you've already been in the room longer than the specified time, the automatic exit time won't change.
:::

## !more

### Extend automatic exit time

Extend your work time.
Specifically, this extends the scheduled automatic exit time.
You can extend the automatic exit time up to 360 minutes from the current time when using this command.

For example, if the scheduled automatic exit time was 30 minutes from now when you use the command, using `!more min=30` would extend the scheduled automatic exit time by another 30 minutes, making it 60 minutes from now.

```text title="Example: Extend by 100 minutes."
!more m=100
```

```text title="Example: Extend by 20 minutes."
!more m=20
```

:::warning
This doesn't mean "automatically exit after ~ minutes from now," but rather "extend the scheduled automatic exit time by ~ minutes."
:::

:::tip
You can omit `min=` with `!more`.

```text title="Example: Extend by 30 minutes."
!more 30
```

:::

:::info
Not only `!more`, but `!okawari` can also be used as the same command. Use whichever you prefer.
:::

## !order

### Order menu items

Choose your favorite item from the menu on screen and order it with this command.
The ordered item will be displayed in the top left of your seat on screen.

```text title="Example: Order item 3"
!order 3
```

:::warning
There is a limit to how many times you can order per day.
:::

:::info
YouTube members can order without limits.
:::

### Remove ordered items

If you want to remove the item displayed at your seat, specify `-` (hyphen) instead of a number.

```text
!order -
``` 
