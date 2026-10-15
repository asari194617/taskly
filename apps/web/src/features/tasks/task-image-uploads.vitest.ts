import { afterEach, expect, it, vi } from "vitest";
import type { TaskFormImage } from "./TaskFormFields";
vi.mock("./api", () => ({ uploadTaskImage: vi.fn() }));
import { uploadTaskImage } from "./api";
import { uploadTaskImages } from "./task-image-uploads";

const upload = vi.mocked(uploadTaskImage);
afterEach(() => vi.resetAllMocks());
const image = (id: string): TaskFormImage => ({
  id,
  file: new File([id], `${id}.png`, { type: "image/png" }),
});

it("preserves image order and reuses uploaded keys", async () => {
  const images = [
    image("first"),
    { id: "existing", key: "existing-key" },
    image("last"),
  ];
  upload.mockResolvedValueOnce("first-key").mockResolvedValueOnce("last-key");
  const update = vi.fn();
  await expect(uploadTaskImages(images, update)).resolves.toEqual([
    "first-key",
    "existing-key",
    "last-key",
  ]);
  expect(upload).toHaveBeenCalledTimes(2);
  expect(update).toHaveBeenCalledWith("first", {
    key: "first-key",
    upload: { state: "done", progress: 100 },
  });
});

it("waits for all images on failure and retries only failed images", async () => {
  let finish!: (key: string) => void;
  let images = [image("failed"), image("slow")];
  const update = (id: string, patch: Partial<TaskFormImage>) => {
    images = images.map((item) =>
      item.id === id ? { ...item, ...patch } : item,
    );
  };
  upload.mockRejectedValueOnce(new Error("offline")).mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  let settled = false;
  const result = uploadTaskImages(images, update).finally(() => {
    settled = true;
  });
  const rejected = expect(result).rejects.toThrow("offline");
  await vi.waitFor(() => expect(images[0].upload?.state).toBe("error"));
  expect(settled).toBe(false);
  finish("slow-key");
  await rejected;
  expect(images[1].key).toBe("slow-key");
  upload.mockResolvedValueOnce("retry-key");
  await expect(uploadTaskImages(images, update)).resolves.toEqual([
    "retry-key",
    "slow-key",
  ]);
  expect(upload).toHaveBeenCalledTimes(3);
});
