import { test } from "../test-setup";
import { expectSampleThumbnailsListed, gotoDemoLanding } from "../demo-landing";

test("demo landing lists sample thumbnails", async ({ page }) => {
  await gotoDemoLanding(page);
  await expectSampleThumbnailsListed(page);
});
