# Tavern Shelf brand assets

`shelf-keeper.png` is the canonical transparent chibi mascot master.
`app-icon.png` is the 512px application icon generated from that master.
`brand-mark.png` is the 256px mark used in the web rail and favicon. All sizes
preserve the complete silhouette and alpha channel, without a frame or crop.

The Shelf Keeper is a cute chibi private media archivist with:

- a large head, fluffy silver-white bob, and a small side braid;
- round amber eyes, rosy cheeks, and a simple gold hair clip;
- a warm-ivory outfit with forest-green and gold details; and
- a large character card with a simple portrait silhouette, plus a second
  card peeking out behind it, held protectively against her chest.

Her expression is a small, contented smile: she treasures the collection
entrusted to her. Keep the rounded proportions and clear hugging gesture.

The mascot was generated and iteratively art-directed for Tavern Shelf with
OpenAI image-generation tooling. Do not use unrelated generated characters as
the Shelf Keeper without preserving the identity traits above.

Run the following command on Windows after changing the mascot master:

```powershell
./scripts/build-brand-assets.ps1
go run ./scripts/generate-windows-icon.go
cd frontend
npm run build
```

The asset script regenerates the app icon, embedded Go icon sizes, the frontend
icon, and an ignored size-preview sheet on dark and light backgrounds under
`build/tools/`. The following commands refresh the Windows ICO and embedded web
assets. Commit these tracked assets together with the master and source changes.
