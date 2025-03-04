---
title: 기본 명령어
sidebar_position: 2
---

## !info

### 작업 기록 표시하기

```
!info
```

누적 작업 시간과 당일 누적 작업 시간을 봇이 알려줍니다.

### 상세 옵션 `d`

```
!info d
```

추가로 다음 정보를 확인할 수 있습니다.

- 랭크 표시 모드 켜기/끄기
- (랭크 표시 모드가 켜진 경우) 랭크 포인트
- (랭크 표시 모드가 켜진 경우) 연속 일수
- 기본 작업 시간
- 즐겨찾기 색상
- 온라인 스터디 스페이스 등록일

## !seat

### 좌석 정보 표시하기

```
!seat
```

현재 앉아 있는 좌석의 다음 정보를 봇이 알려줍니다.

- 현재 앉아 있는 좌석 번호
- 입장 후 경과 시간
- 입장 후 작업 시간(휴식 모드 시간 제외)
- 자동 퇴장까지 남은 시간

### 상세 옵션 `d`

```
!seat d
```

추가로 현재 좌석에 최근 얼마나 오래 앉아 있었는지 확인할 수 있습니다.

## !change

작업명 또는 입장 시간을 변경합니다.
입장 중에만 사용할 수 있는 명령어입니다.
옵션을 1개 이상 지정해야 합니다.

휴식 모드 중인 경우 휴식 내용과 휴식 시간을 변경할 수 있습니다.

### 작업명 변경하기

`work=` `w=` 옵션으로 작업명을 변경할 수 있습니다.

```text title="예: 작업명을 영어로 변경하기"
!change work=영어
```

```text title="예: 작업명을 물리로 변경하기"
!change w=물리
```

```text title="예: 작업명 지우기"
!change work=
```

### 입장 시간 변경하기

`min=` `m=` 옵션으로 입장 시간을 변경할 수 있습니다.

```text title="예: 입장 시간을 40분으로 변경합니다. 입장 후 10분이 경과한 경우, 자동 퇴장 시간은 30분 후(=입장 시간으로부터 40분 후)로 설정됩니다."
!change min=40
```

```text title="예: 작업명을 지우고 입장 시간을 5분으로 변경합니다. 예를 들어 입장 후 3분이 경과한 경우, 자동 퇴장 시간은 2분 후(=입장 시간으로부터 5분 후)로 설정됩니다."
!change w= m=5
```

:::info
이미 지정한 시간 이상 입장한 경우, 자동 퇴장 시간은 변경되지 않습니다.
:::

## !more

### 자동 퇴장 시간 연장하기

작업 시간을 연장합니다.
정확히는 자동 퇴장 예정 시간을 연장합니다.
이 명령어를 사용하는 현재 시간으로부터 360분 후까지 자동 퇴장 시간을 연장할 수 있습니다.

예를 들어, 명령어를 작성한 시점의 자동 퇴장 예정 시간이 30분 후였다면, `!more min=30`에 의해 자동 퇴장 예정 시간이 추가로 30분 연장되어 자동 퇴장 예정 시간은 60분 후가 됩니다.

```text title="예: 100분 연장하기"
!more m=100
```

```text title="예: 20분 연장하기"
!more m=20
```

:::warning
"현재 시간으로부터 ~분 후에 자동 퇴장한다"가 아니라 "자동 퇴장 예정 시간을 ~분 연장한다"는 의미입니다.
:::

:::tip
`!more`에서는 `min=`을 생략할 수 있습니다.

```text title="예: 30분 연장하기"
!more 30
```

:::

:::info
`!more` 외에도 `!okawari`도 같은 명령어로 사용할 수 있습니다. 원하는 것을 사용하세요.
:::

## !order

### 메뉴 주문하기

화면의 메뉴표에서 원하는 아이템을 선택하여 이 명령어로 주문합니다.
주문한 아이템은 화면상 좌석의 왼쪽 상단에 표시됩니다.

```text title="예: 아이템 3 주문하기"
!order 3
```

:::warning
하루에 주문할 수 있는 횟수에는 제한이 있습니다.
:::

:::info
YouTube 멤버는 제한 없이 주문할 수 있습니다.
:::

### 주문한 아이템 치우기

좌석에 표시된 아이템을 지우고 싶을 때는 번호 대신 `-`(하이픈)을 지정합니다.

```text
!order -
``` 
