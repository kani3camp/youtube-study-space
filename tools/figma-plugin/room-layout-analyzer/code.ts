const POSITION_DECIMAL_PLACES = 10;

figma.showUI(__html__, { width: 400, height: 300 });

figma.ui.onmessage = async (msg) => {
  if (msg.type === 'getFrames') {
    try {
      const frameList = getFramesInCurrentPage();
      figma.ui.postMessage({ type: 'frameList', frames: frameList });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      figma.notify(message, { error: true });
      figma.ui.postMessage({ type: 'error', message });
    }
  }
};

type RoomLayoutFrame = {
  frameName: string,
  layoutStr: string,
}

function getFramesInCurrentPage(): RoomLayoutFrame[] {
  const frames = figma.currentPage.findAll((node) => node.type === "FRAME") as Array<FrameNode>;
  const layoutFrames: RoomLayoutFrame[] = [];
  
  type Seat = ComponentNode | InstanceNode;

  function getSeatNumberText(seatGroup: Seat): string {
    const textNode = seatGroup.findChild((node) => node.type === "TEXT");
    if (!textNode || textNode.type !== "TEXT") {
      throw new Error(`Seat "${seatGroup.name}" に座席番号のTEXTノードが見つかりません`);
    }
    return (textNode as TextNode).characters;
  }
  
  for (const frame of frames) {
    console.log('\n' + frame.name);
    
    const seatGroups = frame.findAll((node) => (node.type === "COMPONENT" || node.type === "INSTANCE") && node.name === "Seat") as Array<Seat>;
    seatGroups.sort((a: Seat, b: Seat) => {
      return parseInt(getSeatNumberText(a)) - parseInt(getSeatNumberText(b));
    });
    for (const seatGroup of seatGroups) {
      const seatNumStr = getSeatNumberText(seatGroup);
      console.log(`${seatNumStr} ${roundToDecimalPlace(seatGroup.x, POSITION_DECIMAL_PLACES)} ${roundToDecimalPlace(seatGroup.y, POSITION_DECIMAL_PLACES)}`);
    }
    
    // 座席番号の重複チェック
    const seatNumSet = new Set<string>();
    for (const seatGroup of seatGroups) {
      const seatNumStr = getSeatNumberText(seatGroup);
      if (seatNumSet.has(seatNumStr)) {
        throw new Error(`座席番号 ${seatNumStr} が重複しています`);
      }
      seatNumSet.add(seatNumStr);
    }
    
    layoutFrames.push({
      frameName: frame.name,
      layoutStr: seatGroups.map((seatGroup: Seat) => {
        const seatNumStr = getSeatNumberText(seatGroup);
        const seatStr = `{id: ${seatNumStr}, x: ${roundToDecimalPlace(seatGroup.x, POSITION_DECIMAL_PLACES)}, y: ${roundToDecimalPlace(seatGroup.y, POSITION_DECIMAL_PLACES)}, rotate: 0,},`
        return seatStr;
      }).join('\n'),
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
