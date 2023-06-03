const POSITION_DECIMAL_PLACES = 10;

figma.showUI(__html__, { width: 400, height: 300 });

figma.ui.onmessage = async (msg) => {
  if (msg.type === 'getFrames') {
    const frameList = getFramesInCurrentPage();
    figma.ui.postMessage({ type: 'frameList', frames: frameList });
  }
};

type RoomLayoutFrame = {
  frameName: string,
  seats: {
    seatNum: number,
    x: number,
    y: number,
  }[]
}

function getFramesInCurrentPage(): RoomLayoutFrame[] {
  const frames = figma.currentPage.findAll((node) => node.type === "FRAME") as Array<FrameNode>;
  const layoutFrames: RoomLayoutFrame[] = [];
  
  type Seat = ComponentNode | InstanceNode;
  
  for (const frame of frames) {
    console.log('\n' + frame.name);
    
    const seatGroups = frame.findAll((node) => (node.type === "COMPONENT" || node.type === "INSTANCE") && node.name === "Seat") as Array<Seat>;
    seatGroups.sort((a: Seat, b: Seat) => {
      const aSeatNumText: TextNode = a.findChild((node) => node.type === "TEXT") as TextNode;
      const bSeatNumText: TextNode = b.findChild((node) => node.type === "TEXT") as TextNode;
      return parseInt(aSeatNumText.characters) - parseInt(bSeatNumText.characters);
    });
    for (const seatGroup of seatGroups) {
      const seatNumText: TextNode = seatGroup.findChild((node) => node.type === "TEXT") as TextNode;
      const seatNumStr = seatNumText.characters;
      console.log(`${seatNumStr} ${roundToDecimalPlace(seatGroup.x, POSITION_DECIMAL_PLACES)} ${roundToDecimalPlace(seatGroup.y, POSITION_DECIMAL_PLACES)}`);
    }
    layoutFrames.push({
      frameName: frame.name,
      seats: seatGroups.map((seatGroup: Seat) => {
        const seatNumText: TextNode = seatGroup.findChild((node) => node.type === "TEXT") as TextNode;
        const seatNumStr = seatNumText.characters;
        return {
          seatNum: parseInt(seatNumStr),
          x: roundToDecimalPlace(seatGroup.x, POSITION_DECIMAL_PLACES),
          y: roundToDecimalPlace(seatGroup.y, POSITION_DECIMAL_PLACES),
        }
      }),
    })
  }

  return layoutFrames;
}

/**
 * Rounds a number to the specified number of decimal places.
 *
 * @param value - The number to be rounded.
 * @param decimalPlaces - The number of decimal places to round to.
 * @returns The rounded number.
 */
function roundToDecimalPlace(value: number, decimalPlaces: number): number {
  const factor = Math.pow(10, decimalPlaces);
  return Math.round(value * factor) / factor;
}