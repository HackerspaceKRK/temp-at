// Maps an OUI vendor name (as returned by the backend, e.g. "Espressif Inc.",
// "Hewlett Packard", "Raspberry Pi Foundation") to a bundled logo, or null when
// no logo is known. Matching is a case-insensitive substring check against the
// vendor string, first match wins.

const VENDOR_LOGOS: Array<[keyword: string, file: string]> = [
  ["espressif", "espressif.png"],
  ["raspberry", "raspberry.png"],
  ["mikrotik", "mikrotik.png"],
  ["routerboard", "mikrotik.png"],
  ["hewlett", "hp.png"],
  ["realtek", "realtek.png"],
  ["intel", "intel.png"],
  ["samsung", "samsung.png"],
  ["huawei", "huawei.png"],
  ["xiaomi", "xiaomi.png"],
  ["tp-link", "tp-link.png"],
  ["tplink", "tp-link.png"],
  ["u-blox", "ublox.png"],
  ["ublox", "ublox.png"],
  ["apple", "apple.png"],
  ["asus", "asus.png"],
  ["dell", "dell.png"],
  ["google", "google.png"],
  ["sony", "sony.png"],
  ["brother", "brother.png"],
  ["cisco", "cisco.png"],
  ["dahua", "dahua.png"],
  ["ubiquiti", "ubiquiti.png"],
  ["akuvox", "akuvox.png"],
  ["ampak", "ampak.png"],
  ["nortel", "nortel.png"],
  ["sennheiser", "sennheiser.png"],
  ["zyxel", "zyxel.png"],
];

/** Returns a logo URL for the given vendor name, or null if none matches. */
export function vendorLogoUrl(vendor: string): string | null {
  if (!vendor) return null;
  const v = vendor.toLowerCase();
  for (const [keyword, file] of VENDOR_LOGOS) {
    if (v.includes(keyword)) {
      return `/assets/logos/${file}`;
    }
  }
  return null;
}
