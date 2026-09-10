import { test, expect } from "../test-setup";
import { warmDemoFile, waitForEditorReady, waitForEditorEditable } from "../editor";

test.describe.configure({ mode: "parallel" });

const STATUS_OK_TIMEOUT = 8_000;

for (const [file, editor] of [
  ["sample-files/sample.ods", "cell" as const],
  ["sample-files/sample.xlsx", "cell" as const],
]) {
  test(`content-ready after document-ready: ${file}`, async ({ page, request }) => {
    await warmDemoFile(request, file);
    await page.goto(`/demo/view?file=${encodeURIComponent(file)}`);
    await expect(page.locator("#status")).not.toContainText(/^Error:/, {
      timeout: STATUS_OK_TIMEOUT,
    });

    await waitForEditorReady(page, editor);
    await waitForEditorEditable(page, editor);

    await expect(page.locator("body")).toHaveAttribute("data-content-ready", "true");
  });
}
