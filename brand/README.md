# Tavern Shelf brand assets

`shelf-keeper.png` is the canonical V0 mascot master. `app-icon.png` is the
rounded application-icon crop generated from that master. `brand-mark.png` is
the closer optical crop used in the narrow web rail and favicon.

The Shelf Keeper is a young adult private media archivist with:

- airy shoulder-length silver-white hair and a small side braid;
- amber eyes and a miniature bookshelf hair clip;
- a warm-ivory blouse and charcoal archivist waistcoat with amber details;
- a slender silhouette; and
- three large, full-bleed character-card PNG prints held protectively against
  her chest.

Her canonical V0 expression is a mild affectionate sulk: she has already
brought several cards and is teasing the viewer for still not choosing one.

The mascot was generated and iteratively art-directed for Tavern Shelf with
OpenAI image-generation tooling. Do not use unrelated generated characters as
the Shelf Keeper without preserving the identity traits above.

Run the following command on Windows after changing the mascot master:

```powershell
./scripts/build-brand-assets.ps1
```

The script regenerates the app icon, embedded Go icon sizes, the frontend icon,
and an ignored size-preview sheet under `build/tools/`.
