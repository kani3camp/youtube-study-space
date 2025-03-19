---
title: Essential Commands
sidebar_position: 1
---

## Introduction

Commands start with `!`.

## !in

### Enter the room

```
!in
```

Enter the room and start working.

Your seat will be randomly selected.


### Sit at a specific seat

You can start working at a specified seat by using `!seat_number`.

```text title="Example: Sit at seat number 54."
!54
```

### Sit at the lowest-numbered available seat

```
!0
```

Sit at the lowest-numbered available seat and start working.

### Sit at a members-only seat

```
/in
```

Enter the members-only room.
In this case only, the command starts with `/` instead of `!`.

:::info
Only [YouTube members](https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/join) can sit at members-only seats.
:::

:::warning
It's `/in`, not `!/in`.
:::

```text title="Example: Sit at the lowest-numbered available members-only seat and start working."
/0
```

```text title="Example: Sit at members-only seat number 3."
/3
```

### Specify your work name {#work-option}

You can start working with a specified work name by adding the `work` option.
Please specify your work content (subject, etc.) freely.
Both full-width and half-width characters are acceptable.

```text title="Example: Study English."
!in work English
```

Work names can include spaces.

```text title="Example: Set work name to 'Send Email'."
!in work Send Email
```

If you want to use a work name that starts with `work `, you cannot omit the option name.

```text title="Example: Set work name to 'work plan'."
!in work work plan
```

:::tip
You can omit the option name (`work` or `w`) for work names.

```text title="Example: Study English."
!in English
```

:::

:::info
Work names are OK for chat messages as well.
However, please don't use inappropriate words as they will be displayed on screen.
:::

### Specify maximum work time {#min-option}

You can specify a maximum work time (in minutes) by adding the `min` option.

You will automatically exit the room after the specified time has passed.
Please specify the time (in minutes) from the start of work until automatic exit.
Use half-width numbers.

```text title="Example: Automatically exit after 30 minutes."
!in min 30
```

The automatic exit is primarily a mechanism to prevent forgetting to exit or staying too long.
Therefore, please exit using the `!out` command as a general rule.
Values can be set between `5` and `360` (integers).
The default value is `120`, so if not specified, you will automatically exit after 120 minutes.

### Specify menu order {#order-option}

You can start working with a specified menu order by adding the `order` option.

```text title="Example: Order menu item 5 while entering the room."
!in order 5
```

:::info

The `work` option, `min` option, and `order` option can be used simultaneously.

```text title="Example: Study English and automatically exit after 180 minutes with menu item 5."
!in work English min 180 order 5
```

:::

:::info
Not only with `!in`, but you can also use `work`, `min`, and `order` with `!seat_number`.

```text title="Example: Sit at seat number 54, study English and automatically exit after 180 minutes with menu item 5."
!54 work English min 180 order 5
```

:::

:::warning
While the `work` option itself can be omitted, the `min` option cannot be omitted.

```text title="Example: Study English and automatically exit after 60 minutes."
!in English min 60
```

:::

:::tip
Option names can be abbreviated. `w` has the same meaning as `work`, `m` has the same meaning as `min`, and `o` has the same meaning as `order`.
:::

:::info
Instead of `!in`, you can also use `!work`. Use whichever you prefer.
:::

## !out

### Exit the room

```
!out
```

Exit the room.
