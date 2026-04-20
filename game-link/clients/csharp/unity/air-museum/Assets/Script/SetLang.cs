// SetLang.cs - 多國語言設定（單一功能）
// 使用方式：
//   1. 在場景中放一個空 GameObject，掛上 SetLang。
//   2. 取翻譯：SetLang.T("facility.main")
//   3. 切語言：SetLang.Instance.SetLanguage(Language.EnUS)
using System;
using System.Collections.Generic;
using System.Text;
using UnityEngine;

public enum Language
{
    ZhHant = 0,
    EnUS = 1,
}

public class SetLang : MonoBehaviour
{
    private const string PrefKey = "AirMuseum.Language";

    [Tooltip("Resources 下 lan.csv 的路徑（不含副檔名）")]
    [SerializeField] private string resourcePath = "lan";

    [Tooltip("預設語言")]
    [SerializeField] private Language defaultLanguage = Language.ZhHant;

    [Tooltip("是否記住玩家選的語言")]
    [SerializeField] private bool persist = true;

    public static SetLang Instance { get; private set; }

    /// <summary>語言變更時觸發</summary>
    public event Action<Language> OnLanguageChanged;

    private readonly Dictionary<Language, Dictionary<string, string>> _dict =
        new Dictionary<Language, Dictionary<string, string>>();

    private Language _current;

    /// <summary>目前使用的語言</summary>
    public Language CurrentLanguage => _current;

    private void Awake()
    {
        if (Instance != null && Instance != this)
        {
            Destroy(gameObject);
            return;
        }
        Instance = this;
        DontDestroyOnLoad(gameObject);

        Load();

        _current = persist
            ? (Language)PlayerPrefs.GetInt(PrefKey, (int)defaultLanguage)
            : defaultLanguage;
    }

    private void OnDestroy()
    {
        if (Instance == this) Instance = null;
    }

    /// <summary>切換語言（會觸發 OnLanguageChanged）。</summary>
    public void SetLanguage(Language lang)
    {
        if (_current == lang) return;
        _current = lang;
        if (persist)
        {
            PlayerPrefs.SetInt(PrefKey, (int)lang);
            PlayerPrefs.Save();
        }
        OnLanguageChanged?.Invoke(lang);
    }

    /// <summary>取得 key 對應的翻譯；找不到時回傳 key。</summary>
    public string Get(string key)
    {
        if (string.IsNullOrEmpty(key)) return string.Empty;

        if (_dict.TryGetValue(_current, out var map) && map.TryGetValue(key, out var v))
            return v;

        if (_current != Language.ZhHant &&
            _dict.TryGetValue(Language.ZhHant, out var zh) &&
            zh.TryGetValue(key, out var zhv))
            return zhv;

        return key;
    }

    /// <summary>靜態捷徑：SetLang.T("key")。若 SetLang 未初始化會回傳 key。</summary>
    public static string T(string key)
    {
        return Instance != null ? Instance.Get(key) : key;
    }

    // =====================================================================
    // CSV 載入
    // =====================================================================

    private void Load()
    {
        _dict.Clear();

        var csv = Resources.Load<TextAsset>(resourcePath);
        if (csv == null)
        {
            Debug.LogWarning($"[SetLang] 找不到語言檔 Resources/{resourcePath}.csv");
            return;
        }

        var lines = csv.text.Split(new[] { "\r\n", "\r", "\n" }, StringSplitOptions.None);
        if (lines.Length == 0) return;

        var header = ParseLine(lines[0]);
        var columnLangs = new List<Language>();
        for (int i = 1; i < header.Count; i++)
        {
            var lang = CodeToLanguage(header[i].Trim());
            columnLangs.Add(lang);
            if (!_dict.ContainsKey(lang))
                _dict[lang] = new Dictionary<string, string>();
        }

        for (int r = 1; r < lines.Length; r++)
        {
            if (string.IsNullOrWhiteSpace(lines[r])) continue;

            var cols = ParseLine(lines[r]);
            if (cols.Count == 0) continue;

            var key = cols[0].Trim();
            if (string.IsNullOrEmpty(key)) continue;

            for (int i = 0; i < columnLangs.Count; i++)
            {
                int idx = i + 1;
                if (idx >= cols.Count) break;
                _dict[columnLangs[i]][key] = cols[idx];
            }
        }
    }

    /// <summary>CSV 單行解析，支援以雙引號包起來含逗號的欄位。</summary>
    private static List<string> ParseLine(string line)
    {
        var result = new List<string>();
        if (line == null) return result;

        var sb = new StringBuilder();
        bool inQuotes = false;
        for (int i = 0; i < line.Length; i++)
        {
            char c = line[i];
            if (inQuotes)
            {
                if (c == '"')
                {
                    if (i + 1 < line.Length && line[i + 1] == '"')
                    {
                        sb.Append('"');
                        i++;
                    }
                    else
                    {
                        inQuotes = false;
                    }
                }
                else
                {
                    sb.Append(c);
                }
            }
            else
            {
                if (c == ',')
                {
                    result.Add(sb.ToString());
                    sb.Length = 0;
                }
                else if (c == '"' && sb.Length == 0)
                {
                    inQuotes = true;
                }
                else
                {
                    sb.Append(c);
                }
            }
        }
        result.Add(sb.ToString());
        return result;
    }

    private static Language CodeToLanguage(string code)
    {
        switch (code)
        {
            case "zh-Hant": return Language.ZhHant;
            case "en-US": return Language.EnUS;
            default: return Language.ZhHant;
        }
    }
}
