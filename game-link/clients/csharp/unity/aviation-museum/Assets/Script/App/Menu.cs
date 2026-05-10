using System;
using UnityEngine;
using UnityEngine.EventSystems;
using UnityEngine.UI;

public class Menu : MonoBehaviour
{
    [Serializable]
    public class MenuButton
    {
        public Button button;
        public Image image;
        public Sprite normalSprite;
        public Sprite pressedSprite;

        public Text label;
        public Color normalColor = Color.white;
        public Color pressedColor = Color.white;
    }

    public MenuButton[] buttons;

    void Start()
    {
        if (buttons == null) return;

        for (int i = 0; i < buttons.Length; i++)
        {
            int index = i;
            var item = buttons[i];
            if (item == null || item.button == null) continue;

            var trigger = item.button.gameObject.GetComponent<EventTrigger>();
            if (trigger == null)
            {
                trigger = item.button.gameObject.AddComponent<EventTrigger>();
            }
            trigger.triggers.Clear();

            AddTrigger(trigger, EventTriggerType.PointerDown, _ => SetPressed(index, true));
            AddTrigger(trigger, EventTriggerType.PointerUp, _ => SetPressed(index, false));
            AddTrigger(trigger, EventTriggerType.PointerExit, _ => SetPressed(index, false));

            ApplyState(i, false);
        }
    }

    private void AddTrigger(EventTrigger trigger, EventTriggerType type, Action<BaseEventData> callback)
    {
        var entry = new EventTrigger.Entry { eventID = type };
        entry.callback.AddListener(new UnityEngine.Events.UnityAction<BaseEventData>(callback));
        trigger.triggers.Add(entry);
    }

    public void SetPressed(int index, bool pressed)
    {
        if (buttons == null || index < 0 || index >= buttons.Length) return;
        ApplyState(index, pressed);
    }

    private void ApplyState(int index, bool pressed)
    {
        var item = buttons[index];
        if (item == null) return;

        if (item.image != null)
        {
            Sprite target = pressed ? item.pressedSprite : item.normalSprite;
            if (target != null)
            {
                item.image.sprite = target;
            }
        }

        if (item.label != null)
        {
            item.label.color = pressed ? item.pressedColor : item.normalColor;
        }
    }
}
