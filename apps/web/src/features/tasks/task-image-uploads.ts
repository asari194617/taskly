import { uploadTaskImage } from "./api";
import type { TaskFormImage } from "./TaskFormFields";

export async function uploadTaskImages(
  images: TaskFormImage[],
  update: (id: string, patch: Partial<TaskFormImage>) => void,
): Promise<string[]> {
  // Wait for every upload before unlocking the form, even when one fails.
  const results = await Promise.allSettled(
    images.map(async (image) => {
      if (image.key) return image.key;
      if (!image.file) throw new Error("image_upload_failed");
      try {
        const key = await uploadTaskImage(image.file, (upload) =>
          update(image.id, { upload }),
        );
        update(image.id, { key, upload: { state: "done", progress: 100 } });
        return key;
      } catch (error) {
        update(image.id, { upload: { state: "error" } });
        throw error;
      }
    }),
  );
  const failed = results.find((result) => result.status === "rejected");
  if (failed?.status === "rejected") throw failed.reason;
  return results.map(
    (result) => (result as PromiseFulfilledResult<string>).value,
  );
}
