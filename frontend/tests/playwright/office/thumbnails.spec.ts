import { test } from "../test-setup";
import {
  expectSampleThumbnailsListed,
  expectThumbnailEndpointsOK,
  gotoDemoLanding,
} from "../demo-landing";

test("demo landing lists sample thumbnails", async ({ page, request, checkForErrors }) => {
  await gotoDemoLanding(page);
  await expectSampleThumbnailsListed(page);
  await expectThumbnailEndpointsOK(request, page, 3);
  checkForErrors();
});
