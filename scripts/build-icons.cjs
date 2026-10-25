// Single vector master for Explorer, shortcuts and notification-area icons.
const fs = require('node:fs');
const path = require('node:path');
const sharp = require(process.env.SHARP_PATH || 'sharp');
const root = path.resolve(__dirname, '..');
const sizes = [16, 20, 24, 28, 32, 40, 48, 64, 96, 128, 256];
(async () => {
  const svg = fs.readFileSync(path.join(root, 'Assets/icon.svg'));
  const images = [];
  for (const size of sizes) images.push(await sharp(svg, { density: 1536 }).resize(size, size).png().toBuffer());
  const header = Buffer.alloc(6 + sizes.length * 16);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(sizes.length, 4);
  let offset = header.length;
  sizes.forEach((size, index) => {
    const entry = 6 + index * 16;
    header[entry] = header[entry + 1] = size === 256 ? 0 : size;
    header.writeUInt16LE(1, entry + 4);
    header.writeUInt16LE(32, entry + 6);
    header.writeUInt32LE(images[index].length, entry + 8);
    header.writeUInt32LE(offset, entry + 12);
    offset += images[index].length;
  });
  const ico = Buffer.concat([header, ...images]);
  for (const name of ['senja.ico', 'Assets/senja.ico', 'cmd/samar/senja.ico']) fs.writeFileSync(path.join(root, name), ico);
  await sharp(svg, { density: 1536 }).resize(1024, 1024).png().toFile(path.join(root, 'Assets/icon.png'));
  await sharp(svg, { density: 384 }).resize(128, 128).png().toFile(path.join(root, 'Assets/icon-preview.png'));
  console.log('Built 1024px PNG and 11 native ICO sizes (16–256px).');
})().catch(error => { console.error(error); process.exitCode = 1; });
