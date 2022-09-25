

```mermaid
erDiagram
    USER {
        timestamp LastEntered
        timestamp LastExited
        int totalStudySec
        int dailyTotalStudySec
        bool rankVisible
    }
    SEAT {
        int seatId
        string userId
    }
    
    USER ||--|| SEAT
```