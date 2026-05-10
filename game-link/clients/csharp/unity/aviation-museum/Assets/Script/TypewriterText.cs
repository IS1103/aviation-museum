using System.Collections;
using UnityEngine;
using UnityEngine.UI;

/// <summary>
/// 將目標 <see cref="Text"/> 以打字機方式逐字顯示。若未指定 target，會使用同物件上的 Text。
/// </summary>
public class TypewriterText : MonoBehaviour
{
    [Header("自動播放")]
    [Tooltip("勾選後，每次啟用會讀取 Target Text 目前的字串並逐字打出。若只由程式改字，請關閉並改呼叫 Type(string)。")]
    [SerializeField] private bool playOnEnable = true;

    [SerializeField] private Text targetText;

    [Tooltip("字與字之間的間隔（秒）；約 0.03～0.08 較像打字。設成 1 會很慢且容易以為沒動作。")]
    [SerializeField] private float secondsPerCharacter = 0.05f;

    [Header("開頭游標")]
    [Tooltip("打字前先顯示此字元並閃爍。")]
    [SerializeField] private string cursorChar = "|";

    [Tooltip("游標亮起或熄滅各持續多久（秒）。")]
    [SerializeField] private float cursorBlinkPhaseSeconds = 0.25f;

    [Tooltip("完整「亮→暗」算一次；2 即閃兩次。")]
    [SerializeField] private int cursorBlinkCount = 2;

    [Header("結尾游標")]
    [Tooltip("最後一字打完後，在句尾持續閃爍 |，模擬游標。")]
    [SerializeField] private bool trailingCursorBlink = true;

    private Coroutine _routine;
    private string _fullText = "";

    void Awake()
    {
        if (targetText == null)
            targetText = GetComponent<Text>();
    }

    void OnEnable()
    {
        if (!playOnEnable || targetText == null)
            return;

        Type(targetText.text);
    }

    /// <summary>立刻顯示完整文字並停止進行中的打字效果。</summary>
    public void SetImmediate(string text)
    {
        StopTyping();
        _fullText = text ?? string.Empty;
        if (targetText != null)
            targetText.text = _fullText;
    }

    /// <summary>從空字串開始逐字打出內容。若已有未完成的動畫會先取消。</summary>
    public void Type(string text)
    {
        StopTyping();
        _fullText = text ?? string.Empty;

        if (targetText == null)
            return;

        if (secondsPerCharacter <= 0f)
        {
            _routine = StartCoroutine(TypeRoutineImmediate());
            return;
        }

        _routine = StartCoroutine(TypeRoutine());
    }

    /// <summary>跳到結尾顯示整句（用於略過動畫）。</summary>
    public void FinishImmediately()
    {
        StopTyping();
        if (targetText != null)
            targetText.text = _fullText;

        if (targetText != null && trailingCursorBlink)
            _routine = StartCoroutine(TrailingBlinkLoop());
    }

    public void StopTyping()
    {
        if (_routine != null)
        {
            StopCoroutine(_routine);
            _routine = null;
        }
    }

    void OnDestroy()
    {
        StopTyping();
    }

    private IEnumerator TypeRoutine()
    {
        int len = _fullText.Length;
        targetText.text = string.Empty;

        IEnumerator intro = CursorBlinkRoutine();
        while (intro.MoveNext())
            yield return intro.Current;

        for (int i = 0; i < len; i++)
        {
            targetText.text = _fullText.Substring(0, i + 1);
            if (i + 1 < len)
                yield return new WaitForSeconds(secondsPerCharacter);
        }

        if (trailingCursorBlink)
        {
            IEnumerator tail = TrailingBlinkLoop();
            while (tail.MoveNext())
                yield return tail.Current;
        }
        else
            _routine = null;
    }

    private IEnumerator TypeRoutineImmediate()
    {
        targetText.text = string.Empty;
        IEnumerator intro = CursorBlinkRoutine();
        while (intro.MoveNext())
            yield return intro.Current;

        targetText.text = _fullText;

        if (trailingCursorBlink)
        {
            IEnumerator tail = TrailingBlinkLoop();
            while (tail.MoveNext())
                yield return tail.Current;
        }
        else
            _routine = null;
    }

    /// <summary>句尾循環閃爍游標（銜在全文之後）。</summary>
    private IEnumerator TrailingBlinkLoop()
    {
        string caret = CaretSymbol();
        float phase = Mathf.Max(0.01f, cursorBlinkPhaseSeconds);

        while (true)
        {
            targetText.text = _fullText + caret;
            yield return new WaitForSeconds(phase);
            targetText.text = _fullText;
            yield return new WaitForSeconds(phase);
        }
    }

    /// <summary>游標亮 → 暗，重複 <see cref="cursorBlinkCount"/> 次。</summary>
    private IEnumerator CursorBlinkRoutine()
    {
        string caret = CaretSymbol();
        float phase = Mathf.Max(0.01f, cursorBlinkPhaseSeconds);
        int times = Mathf.Max(0, cursorBlinkCount);

        for (int b = 0; b < times; b++)
        {
            targetText.text = caret;
            yield return new WaitForSeconds(phase);
            targetText.text = string.Empty;
            yield return new WaitForSeconds(phase);
        }
    }

    private string CaretSymbol()
    {
        return string.IsNullOrEmpty(cursorChar) ? "|" : cursorChar;
    }
}
