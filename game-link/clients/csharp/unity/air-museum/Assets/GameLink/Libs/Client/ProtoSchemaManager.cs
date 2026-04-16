// ProtoSchemaManager.cs - Protobuf 型別解析（對應 Cocos ProtoSchemaManager）
using System;
using System.Collections.Generic;
using Google.Protobuf;
using Gate;
using AirMuseum;
using UnityEngine;

namespace GameLink.Libs.Client
{
    /// <summary>依 typeName 解析 Any 的 value 為 IMessage</summary>
    public static class ProtoSchemaManager
    {
        public const int DefaultTimeoutMs = 15000;
        public const int RetryIntervalMs = 2000;

        /// <summary>為 true 時，TryParse 失敗會打 log（找不到 parser 或解析拋錯），方便排查 Info 解析為 null 的原因。</summary>
        public static bool EnableParseFailureLog = false;

        private static readonly Dictionary<string, Func<ByteString, IMessage>> Parsers =
            new Dictionary<string, Func<ByteString, IMessage>>();

        static ProtoSchemaManager()
        {
            RegisterGate();
            RegisterAirMuseum();
        }

        private static void RegisterGate()
        {
            Register(Pack.Descriptor.FullName, bs => Pack.Parser.ParseFrom(bs));
            Register(ValidateReq.Descriptor.FullName, bs => ValidateReq.Parser.ParseFrom(bs));
            Register(ValidateResp.Descriptor.FullName, bs => ValidateResp.Parser.ParseFrom(bs));
            Register(PingReq.Descriptor.FullName, bs => PingReq.Parser.ParseFrom(bs));
            Register(PingResp.Descriptor.FullName, bs => PingResp.Parser.ParseFrom(bs));
            Register(ErrorDetail.Descriptor.FullName, bs => ErrorDetail.Parser.ParseFrom(bs));
        }

        private static void RegisterAirMuseum()
        {
            Register(AirMuseum.GameState.Descriptor.FullName, bs => AirMuseum.GameState.Parser.ParseFrom(bs));
            Register(AirMuseum.PlayerInput.Descriptor.FullName, bs => AirMuseum.PlayerInput.Parser.ParseFrom(bs));
            Register(AirMuseum.ErrorNotify.Descriptor.FullName, bs => AirMuseum.ErrorNotify.Parser.ParseFrom(bs));
        }

        private static void Register(string fullName, Func<ByteString, IMessage> parser)
        {
            if (parser == null) return;
            Parsers[fullName] = parser;
        }

        /// <summary>註冊額外 proto 型別（遊戲專用等）</summary>
        public static void RegisterProto(string fullName, Func<ByteString, IMessage> parser)
        {
            Register(fullName, parser);
        }

        /// <summary>依完整型別名解析 bytes 為 IMessage（TypeUrl 可為 type.googleapis.com/xxx）</summary>
        /// <remarks>先精確比對，再依不區分大小寫比對。回傳 false（message=null）可能原因：① 未註冊該 TypeUrl ② Info.Value 非該型別或損壞導致 ParseFrom 拋錯 ③ fullTypeName/bytes 為空。</remarks>
        public static bool TryParse(string fullTypeName, ByteString bytes, out IMessage message)
        {
            message = null;
            if (string.IsNullOrEmpty(fullTypeName) || bytes == null)
            {
                if (EnableParseFailureLog)
                    Debug.LogWarning($"[ProtoSchemaManager] TryParse 跳過：fullTypeName 或 bytes 為空");
                return false;
            }
            var name = fullTypeName;
            if (name.StartsWith("type.googleapis.com/", StringComparison.Ordinal))
                name = name.Substring("type.googleapis.com/".Length);
            if (!TryGetParser(name, out var parser))
            {
                if (EnableParseFailureLog)
                    Debug.LogWarning($"[ProtoSchemaManager] TryParse 失敗：未註冊 TypeUrl name=\"{name}\"（原始 fullTypeName=\"{fullTypeName}\"）");
                return false;
            }
            try
            {
                message = parser(bytes);
                return true;
            }
            catch (Exception ex)
            {
                if (EnableParseFailureLog)
                    Debug.LogWarning($"[ProtoSchemaManager] TryParse 失敗：ParseFrom 拋錯 name=\"{name}\" ValueLen={bytes?.Length ?? 0} ex={ex.Message}");
                return false;
            }
        }

        private static bool TryGetParser(string name, out Func<ByteString, IMessage> parser)
        {
            parser = null;
            if (Parsers.TryGetValue(name, out parser)) return true;
            foreach (var kv in Parsers)
            {
                if (string.Equals(kv.Key, name, StringComparison.OrdinalIgnoreCase))
                {
                    parser = kv.Value;
                    return true;
                }
            }
            return false;
        }

        /// <summary>將 Any.Value 轉為 byte[]</summary>
        public static byte[] ValueToBytes(Google.Protobuf.WellKnownTypes.Any any)
        {
            if (any?.Value == null || any.Value.Length == 0)
                return Array.Empty<byte>();
            return any.Value.ToByteArray();
        }
    }
}
