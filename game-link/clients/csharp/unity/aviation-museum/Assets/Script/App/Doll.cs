using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

public class Doll : MonoBehaviour
{
    [Header("角色")]
    [Tooltip("要控制的 Man 角色物件")]
    [SerializeField] private Man man;

    [Header("流程按鈕")]
    [Tooltip("確認按鈕：關閉 this 並開啟 playerObj")]
    [SerializeField] private Button confirmButton;
    [Tooltip("自定服裝按鈕：關閉 this 並開啟 dressUpHouseObj")]
    [SerializeField] private Button customClothesButton;

    [Header("流程目標 GameObject")]
    [Tooltip("按下「確認」後要開啟的 Player GameObject")]
    [SerializeField] private GameObject playerObj;
    [Tooltip("按下「自定服裝」後要開啟的變裝屋 GameObject")]
    [SerializeField] private GameObject dressUpHouseObj;

    [Header("粒子效果")]
    [Tooltip("初始為關閉；呼叫 PlayParticleEffects 時以隨機順序播放，相鄰兩個間隔見下方")]
    [SerializeField] private ParticleSystem[] particleEffects;
    [Tooltip("隨機排序後，相鄰兩個粒子開始播放的間隔（秒）")]
    [SerializeField] private float particleEffectStaggerSeconds = 0.3f;

    private Coroutine particleEffectsRoutine;

    private void Awake()
    {
        StopParticleEffects();
    }

    /// <summary>
    /// 停止並清除陣列內所有粒子（開場狀態：不播放）。
    /// </summary>
    public void StopParticleEffects()
    {
        if (particleEffectsRoutine != null)
        {
            StopCoroutine(particleEffectsRoutine);
            particleEffectsRoutine = null;
        }
        if (particleEffects == null)
            return;
        foreach (var ps in particleEffects)
            StopAndClear(ps);
    }

    /// <summary>
    /// 將陣列中非 null 的粒子隨機排序後依序播放，相鄰兩個間隔 <see cref="particleEffectStaggerSeconds"/> 秒。
    /// </summary>
    public void PlayParticleEffects()
    {
        if (particleEffectsRoutine != null)
            StopCoroutine(particleEffectsRoutine);
        particleEffectsRoutine = StartCoroutine(PlayParticleEffectsRoutine());
    }

    private IEnumerator PlayParticleEffectsRoutine()
    {
        var list = BuildNonNullParticleList();
        if (list.Count == 0)
        {
            particleEffectsRoutine = null;
            yield break;
        }

        Shuffle(list);

        for (var i = 0; i < list.Count; i++)
        {
            var ps = list[i];
            ps.gameObject.SetActive(true);
            ps.Play();
            if (i < list.Count - 1)
                yield return new WaitForSeconds(particleEffectStaggerSeconds);
        }

        particleEffectsRoutine = null;
    }

    private List<ParticleSystem> BuildNonNullParticleList()
    {
        var list = new List<ParticleSystem>();
        if (particleEffects == null)
            return list;
        foreach (var ps in particleEffects)
        {
            if (ps != null)
                list.Add(ps);
        }
        return list;
    }

    private static void Shuffle(List<ParticleSystem> list)
    {
        for (var i = list.Count - 1; i > 0; i--)
        {
            var j = Random.Range(0, i + 1);
            (list[i], list[j]) = (list[j], list[i]);
        }
    }

    private static void StopAndClear(ParticleSystem ps)
    {
        if (ps == null)
            return;
        ps.Stop(true, ParticleSystemStopBehavior.StopEmittingAndClear);
        ps.Clear();
    }

    private void OnEnable()
    {
        if (confirmButton != null)
            confirmButton.onClick.AddListener(OnConfirmClicked);
        if (customClothesButton != null)
            customClothesButton.onClick.AddListener(OnCustomClothesClicked);

        PlayParticleEffects();
    }

    private void OnDisable()
    {
        if (confirmButton != null)
            confirmButton.onClick.RemoveListener(OnConfirmClicked);
        if (customClothesButton != null)
            customClothesButton.onClick.RemoveListener(OnCustomClothesClicked);
    }

    private void OnConfirmClicked()
    {
        SwitchTo(playerObj);
    }

    private void OnCustomClothesClicked()
    {
        SwitchTo(dressUpHouseObj);
    }

    private void SwitchTo(GameObject target)
    {
        if (target != null)
        {
            target.SetActive(true);
        }
        gameObject.SetActive(false);
    }

    void Start()
    {
        int uid = PlayerPrefs.GetInt("air_museum_uid", 0);
        string name = PlayerPrefs.GetString("air_museum_name", "");
        int regAge = PlayerPrefs.GetInt("air_museum_age", 0);
        int sex = PlayerPrefs.GetInt("air_museum_sex", -1);

        int eyesIndex = PlayerPrefs.GetInt("air_museum_eyes_index", 0);
        int eyebrowIndex = PlayerPrefs.GetInt("air_museum_eyebrow_index", 0);
        int mouthIndex = PlayerPrefs.GetInt("air_museum_mouth_index", 0);
        int glassesIndex = PlayerPrefs.GetInt("air_museum_glasses_index", -1);
        int helmetIndex = PlayerPrefs.GetInt("air_museum_helmet_index", 0);

        man.SetEyes(eyesIndex);
        man.SetEyebrow(eyebrowIndex);
        man.SetMouth(mouthIndex);
        man.SetGlasses(glassesIndex);
        man.SetHelmet(helmetIndex);

        Debug.Log(
            "[Doll] PlayerPrefs 狀態：\n" +
            $"  註冊資料  → uid={uid}, name=\"{name}\", age={regAge}, sex={sex}\n" +
            $"  模型編號  → eyes={eyesIndex}, mouth={mouthIndex}, glasses={glassesIndex}, helmet={helmetIndex}"
        );
    }
}
