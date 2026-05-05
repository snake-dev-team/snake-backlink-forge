"use client";

export type TonePreference = "professional" | "casual" | "storytelling" | "technical";

const TONE_OPTIONS: { value: TonePreference; label: string; description: string }[] = [
  { value: "professional", label: "Professional", description: "Formal, authoritative tone" },
  { value: "casual", label: "Casual", description: "Conversational, friendly tone" },
  { value: "storytelling", label: "Storytelling", description: "Narrative-driven content" },
  { value: "technical", label: "Technical", description: "Detail-focused, precise language" },
];

type Props = {
  value: TonePreference;
  onChange: (v: TonePreference) => void;
};

/**
 * ToneSelect — native select for AI content tone preference.
 * Matches UI B aesthetic using border-input bg-background classes.
 */
export function ToneSelect({ value, onChange }: Props) {
  return (
    <div className="grid gap-2">
      <label htmlFor="tone_preference" className="text-sm font-medium">
        Tone preference
      </label>
      <select
        id="tone_preference"
        value={value}
        onChange={(e) => onChange(e.target.value as TonePreference)}
        className="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
      >
        {TONE_OPTIONS.map((opt) => (
          <option key={opt.value} value={opt.value} title={opt.description}>
            {opt.label} — {opt.description}
          </option>
        ))}
      </select>
    </div>
  );
}
