using UnityEngine;

public class TestBird : MonoBehaviour
{
    [Header("動畫目標 (留空則使用自身或子物件上的 Animator)")]
    [Tooltip("要播放動畫的 Animator，若未指派會自動從自身或子物件搜尋")]
    public Animator animator;

    [Header("動畫名稱")]
    [Tooltip("Animator Controller 中的 State 名稱，需與模型動畫一致 (例如 Fly / Idle_A / Walk)")]
    public string animationName = "Fly";

    void Start()
    {
        if (animator == null)
        {
            animator = GetComponent<Animator>();
        }

        if (animator == null)
        {
            animator = GetComponentInChildren<Animator>();
        }

        PlayAnimation(animationName);
    }

    public void PlayAnimation(string stateName)
    {
        if (animator == null)
        {
            Debug.LogWarning("[TestBird] 找不到 Animator 元件，無法播放動畫: " + stateName);
            return;
        }

        if (!animator.HasState(0, Animator.StringToHash(stateName)))
        {
            Debug.LogWarning("[TestBird] Animator 中找不到 State: " + stateName);
            return;
        }

        animator.Play(stateName);
    }

    void Update()
    {
        
    }
}
