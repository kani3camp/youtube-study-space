---
title: Essential Commands
sidebar_position: 1
---

## Introduction

Commands start with `!`.

:::warning
Please use half-width characters for all input. Use `!` (half-width) instead of `！` (full-width).
:::

## !in

### Enter the room

```
!in
```

Enter the room and start working.

:::info
Your seat will be randomly selected.
:::

### Sit at a specific seat

You can start working at a specified seat by using `!seat_number`.

```text title="Example: Sit at seat number 8."
!8
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
Only YouTube members can sit at members-only seats.
:::

:::warning
It's `/in`, not `!/in`.
:::

### Sit at the lowest-numbered members-only seat

```
/0
```

Sit at the lowest-numbered available members-only seat and start working.

:::tip
You can also specify a number for a members-only seat.

```text title="Example: Sit at members-only seat number 3."
/3
```

:::

### Specify your work name

You can start working with a specified work name by adding the `work=` option.
Please specify your work content (subject, etc.) freely.
Both full-width and half-width characters are acceptable.
Make sure there are no spaces in the work name.

```text title="Example: Study English."
!in work=English
```

:::info
Work names can be used in chat messages as well.
However, please don't use inappropriate words as they will be displayed on screen.
:::

### Specify maximum work time {#min-option}

You can specify a maximum work time (in minutes) by adding the `min=` option.

You will automatically exit the room after the specified time has passed.
Please specify the time (in minutes) from the start of work until automatic exit.
Use half-width numbers.
The automatic exit is primarily a mechanism to prevent forgetting to exit or staying too long.
Therefore, please exit using the `!out` command as a general rule.
Values can be set between `5` and `360` (integers).
The default value is `120`, so if not specified, you will automatically exit after 120 minutes.

```text title="Example: Automatically exit after 30 minutes."
!in min=30
```

:::info

The `work=` option and `min=` option can be used simultaneously.

```text title="Example: Study English and automatically exit after 180 minutes."
!in work=English min=180
```

:::

:::info
Not only with `!in`, but you can also use `work=` and `min=` with `!seat_number`.
:::

:::tip
Additional options can be abbreviated. `w=` has the same meaning as `work=`, and `m=` has the same meaning as `min=`.
:::

### Traditional option specification methods

- `work-work_name`
- `min-maximum_work_time`
- `w-work_name`
- `m-maximum_work_time`

These traditional commands can still be used.

## !out

### Exit the room

```
!out
```

Exit the room. 
