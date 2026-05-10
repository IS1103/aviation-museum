using UnityEngine;
#if ENABLE_INPUT_SYSTEM
using UnityEngine.InputSystem;
#endif

public class PlanCtrl : MonoBehaviour
{
    [Header("控制目標 (留空則控制自身)")]
    [Tooltip("要被鍵盤操控的目標物件，若未指派則預設使用此腳本所在的 GameObject")]
    public Transform target;

    [Header("移動參數")]
    [Tooltip("前進速度 (單位/秒)")]
    public float forwardSpeed = 5f;

    [Tooltip("左右移動速度 (單位/秒)")]
    public float strafeSpeed = 3f;

#if ENABLE_LEGACY_INPUT_MANAGER
    [Header("輸入按鍵設定 (舊 Input Manager)")]
    [Tooltip("前進按鍵")]
    public KeyCode forwardKey = KeyCode.W;

    [Tooltip("後退按鍵")]
    public KeyCode backwardKey = KeyCode.S;

    [Tooltip("向左移動按鍵")]
    public KeyCode leftKey = KeyCode.A;

    [Tooltip("向右移動按鍵")]
    public KeyCode rightKey = KeyCode.D;
#endif

    void Start()
    {
        if (target == null)
        {
            target = transform;
        }
    }

    void Update()
    {
        if (target == null)
        {
            return;
        }

        float forwardInput = 0f;
        float strafeInput = 0f;

#if ENABLE_INPUT_SYSTEM
        Keyboard kb = Keyboard.current;
        if (kb != null)
        {
            if (kb.wKey.isPressed || kb.upArrowKey.isPressed) forwardInput += 1f;
            if (kb.sKey.isPressed || kb.downArrowKey.isPressed) forwardInput -= 1f;
            if (kb.dKey.isPressed || kb.rightArrowKey.isPressed) strafeInput += 1f;
            if (kb.aKey.isPressed || kb.leftArrowKey.isPressed) strafeInput -= 1f;
        }
#elif ENABLE_LEGACY_INPUT_MANAGER
        if (Input.GetKey(forwardKey)) forwardInput += 1f;
        if (Input.GetKey(backwardKey)) forwardInput -= 1f;
        if (Input.GetKey(rightKey)) strafeInput += 1f;
        if (Input.GetKey(leftKey)) strafeInput -= 1f;
#endif

        Vector3 movement =
            target.forward * forwardInput * forwardSpeed +
            target.right * strafeInput * strafeSpeed;

        target.position += movement * Time.deltaTime;
    }
}
