import { del, get, request } from "@/api/client";
import type { SkillFile, SkillSummary, SkillTree } from "@/models/skillhub";

const SKILLS_PATH = "api/v1/skills";

export function fetchSkills(): Promise<SkillSummary[]> {
  return get<SkillSummary[]>(SKILLS_PATH);
}

export function fetchSkillTree(path = ""): Promise<SkillTree> {
  const params = new URLSearchParams();
  if (path.trim()) {
    params.set("path", path.trim());
  }
  const query = params.toString();
  return get<SkillTree>(`${SKILLS_PATH}/tree${query ? `?${query}` : ""}`);
}

export function fetchSkillFile(path: string): Promise<SkillFile> {
  const params = new URLSearchParams({ path });
  return get<SkillFile>(`${SKILLS_PATH}/file?${params.toString()}`);
}

export function deleteSkillRequest(name: string): Promise<void> {
  return del<void>(`${SKILLS_PATH}/${encodeURIComponent(String(name || "").trim())}`);
}

export function uploadSkillArchive(file: File): Promise<SkillSummary> {
  const formData = new FormData();
  formData.set("file", file);
  return request<SkillSummary>("api/v1/skills:upload", {
    body: formData,
    method: "POST",
  });
}
