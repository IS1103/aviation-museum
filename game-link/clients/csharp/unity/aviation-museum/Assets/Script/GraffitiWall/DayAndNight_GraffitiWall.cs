using UnityEngine;
using UnityEngine.Rendering;
using UnityEngine.Rendering.Universal;
using UnityEngine.UI;

public class DayAndNight_GraffitiWall : MonoBehaviour
{
    [Header("燈光角度 (0 ~ 100)")]
    [Range(0f, 100f)]
    public float lightRotation;

    [Header("Directional Light")]
    public Light directionalLight;

    [Header("Global Volume")]
    public Volume globalVolume;

    [Header("Post Exposure (-3 ~ 1)")]
    [Range(-3f, 1f)]
    public float postExposure;

    [Header("測試用時間")]
    public float testTime;

    [Header("晚上持續時間")]
    public float nightDuration;

    [Header("進入晚上時 PostExposure 淡化時間")]
    public float fadeToNightDuration = 2f;

    [Header("離開晚上時 PostExposure 淡化時間")]
    public float fadeToDayDuration = 2f;

    [Header("夜晚 Image 遮罩")]
    public Image nightOverlayImage;

    [Header("Image 淡入淡出時間")]
    public float imageFadeDuration = 1f;

    private ColorAdjustments colorAdjustments;

    void Start()
    {
        if (globalVolume != null && globalVolume.profile != null)
        {
            globalVolume.profile.TryGet(out colorAdjustments);
        }
    }

    void Update()
    {
        if (testTime <= 0f) return;

        float dayLength = 2f * testTime;
        float safeNight = Mathf.Max(0f, nightDuration);
        float cycleLength = dayLength + safeNight;
        float t = Time.time % cycleLength;

        float ratio;
        if (t < testTime)
        {
            ratio = t / testTime;
        }
        else if (t < dayLength)
        {
            ratio = 1f - (t - testTime) / testTime;
        }
        else
        {
            ratio = 0f;
        }

        lightRotation = ratio * 100f;

        if (t < dayLength)
        {
            postExposure = 1f;
        }
        else
        {
            float nightT = t - dayLength;
            float fadeIn = Mathf.Max(0f, fadeToNightDuration);
            float fadeOut = Mathf.Max(0f, fadeToDayDuration);

            if (fadeIn > 0f && nightT < fadeIn)
            {
                postExposure = Mathf.Lerp(1f, -3f, nightT / fadeIn);
            }
            else if (nightT < safeNight - fadeOut)
            {
                postExposure = -3f;
            }
            else if (fadeOut > 0f)
            {
                float intoFadeOut = nightT - (safeNight - fadeOut);
                postExposure = Mathf.Lerp(-3f, 1f, intoFadeOut / fadeOut);
            }
            else
            {
                postExposure = -3f;
            }
        }

        float overlayAlpha = 0f;
        if (t >= dayLength)
        {
            float nightT = t - dayLength;
            float fadeIn = Mathf.Max(0f, fadeToNightDuration);
            float fadeOut = Mathf.Max(0f, fadeToDayDuration);
            float imgFade = Mathf.Max(0f, imageFadeDuration);

            float imgFadeInStart = 1f;
            float imgFadeInEnd = imgFadeInStart + imgFade;
            float imgFadeOutEnd = safeNight - fadeOut;
            float imgFadeOutStart = imgFadeOutEnd - imgFade;

            if (nightT < imgFadeInStart)
            {
                overlayAlpha = 0f;
            }
            else if (imgFade > 0f && nightT < imgFadeInEnd)
            {
                overlayAlpha = (nightT - imgFadeInStart) / imgFade;
            }
            else if (nightT < imgFadeOutStart)
            {
                overlayAlpha = 1f;
            }
            else if (imgFade > 0f && nightT < imgFadeOutEnd)
            {
                overlayAlpha = 1f - (nightT - imgFadeOutStart) / imgFade;
            }
            else
            {
                overlayAlpha = 0f;
            }
        }

        if (directionalLight != null)
        {
            directionalLight.transform.rotation = Quaternion.Euler(lightRotation, 0f, 0f);

            directionalLight.intensity = ratio * 3f;
        }

        if (colorAdjustments != null)
        {
            colorAdjustments.postExposure.value = postExposure;
        }

        if (nightOverlayImage != null)
        {
            Color c = nightOverlayImage.color;
            c.a = Mathf.Clamp01(overlayAlpha);
            nightOverlayImage.color = c;
        }
    }
}
