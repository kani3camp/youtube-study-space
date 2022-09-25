from typing import List

INPUT_FILE_NAME: str = 'seats_positions.txt'

OFFSET_X = 535
OFFSET_Y = -725

def main():
    with open(INPUT_FILE_NAME, mode='r') as f:
        lines: List[str] = [s.rstrip('\n') for s in f.readlines()]
        # print(lines)
        for line in lines:
            items = line.split()
            assert len(items) == 3
            seat_id = items[0]
            x = int(items[1])
            y = int(items[2])
            print('{', f'id: {seat_id},x:{x+OFFSET_X},y:{y+OFFSET_Y},rotate:0,', '},')


if __name__ == '__main__':
    main()
