// For private use.
// read every seat's position on the adobe illustrator file.
// see api documents: https://ai-scripting.docsforadobe.dev/index.html

const thisDoc = app.activeDocument;

var text = (new Date()).toTimeString().replace(' ', '') + '\n';

text += 'width: ' + thisDoc.width + '\n';
text += 'height: ' + thisDoc.height + '\n';

// 謎のずれを補正する。
const MysteryOffsetX = 880;
const MysteryOffsetY = -440;

const seatsLayer = thisDoc.layers.getByName('Seats');
var i;
text += seatsLayer.pathItems.length + ' items in this layer.\n';
for (i=1; i <= seatsLayer.pathItems.length; i++ ) {
    var item = seatsLayer.pathItems.getByName(i.toString());
    // text += item.name + ' ' + item.position + '\n';
    var x = item.left + MysteryOffsetX;
    var y = thisDoc.height - item.top + MysteryOffsetY;
    text += item.name + ' ' + Math.round(x) + ' ' + Math.round(y) + '\n';
}
// $.writeln(text);
alert(text);
