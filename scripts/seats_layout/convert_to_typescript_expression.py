from typing import List

INPUT_FILE_NAME: str = r'C:\Users\momom\Downloads\Frame 1.txt'

OFFSET_X = 0
OFFSET_Y = 0


def main():
    with open(INPUT_FILE_NAME, mode='r', encoding='utf-8') as f:
        lines: List[str] = [s.rstrip('\n') for s in f.readlines()]
        # print(lines)
        for line in lines:
            items = line.split()
            assert len(items) == 3
            seat_id = items[0]
            x = float(items[1])
            y = float(items[2])
            print('{', f'id: {seat_id},x:{x+OFFSET_X},y:{y+OFFSET_Y},rotate:0,', '},')


if __name__ == '__main__':
    main()
