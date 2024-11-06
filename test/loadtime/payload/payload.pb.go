package payload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPayload_Reset(t *testing.T) {
	tests := []struct {
		name    string
		payload *Payload
	}{
		{
			name: "reset populated payload",
			payload: &Payload{
				Connections: 100,
				Rate:       1000,
				Size:       1024,
				Time:       timestamppb.Now(),
				Id:         []byte("test-id"),
				Padding:    []byte("test-padding"),
			},
		},
		{
			name:    "reset empty payload",
			payload: &Payload{},
		},
		{
			name: "reset partial payload",
			payload: &Payload{
				Connections: 100,
				Time:       timestamppb.Now(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.payload.Reset()
			assert.Equal(t, uint64(0), tc.payload.Connections)
			assert.Equal(t, uint64(0), tc.payload.Rate)
			assert.Equal(t, uint64(0), tc.payload.Size)
			assert.Nil(t, tc.payload.Time)
			assert.Nil(t, tc.payload.Id)
			assert.Nil(t, tc.payload.Padding)
		})
	}
}

func TestPayload_ProtoMethods(t *testing.T) {
	payload := &Payload{
		Connections: 100,
		Rate:       1000,
		Size:       1024,
		Time:       timestamppb.Now(),
		Id:         []byte("test-id"),
		Padding:    []byte("test-padding"),
	}

	t.Run("String", func(t *testing.T) {
		str := payload.String()
		assert.NotEmpty(t, str)
		assert.Contains(t, str, "100")          // Connections
		assert.Contains(t, str, "1000")         // Rate
		assert.Contains(t, str, "1024")         // Size
		assert.Contains(t, str, "test-id")      // Id
		assert.Contains(t, str, "test-padding") // Padding
	})

	t.Run("ProtoMessage", func(t *testing.T) {
		// Should not panic
		payload.ProtoMessage()
	})

	t.Run("ProtoReflect", func(t *testing.T) {
		r := payload.ProtoReflect()
		assert.NotNil(t, r)
		assert.True(t, r.IsValid())
	})
}

func TestPayload_GetMethods(t *testing.T) {
	now := timestamppb.Now()
	tests := []struct {
		name    string
		payload *Payload
		want    *Payload
	}{
		{
			name: "get all fields",
			payload: &Payload{
				Connections: 100,
				Rate:       1000,
				Size:       1024,
				Time:       now,
				Id:         []byte("test-id"),
				Padding:    []byte("test-padding"),
			},
			want: &Payload{
				Connections: 100,
				Rate:       1000,
				Size:       1024,
				Time:       now,
				Id:         []byte("test-id"),
				Padding:    []byte("test-padding"),
			},
		},
		{
			name:    "get from nil payload",
			payload: nil,
			want: &Payload{
				Connections: 0,
				Rate:       0,
				Size:       0,
				Time:       nil,
				Id:         nil,
				Padding:    nil,
			},
		},
		{
			name:    "get from empty payload",
			payload: &Payload{},
			want: &Payload{
				Connections: 0,
				Rate:       0,
				Size:       0,
				Time:       nil,
				Id:         nil,
				Padding:    nil,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.payload != nil {
				assert.Equal(t, tc.want.Connections, tc.payload.GetConnections())
				assert.Equal(t, tc.want.Rate, tc.payload.GetRate())
				assert.Equal(t, tc.want.Size, tc.payload.GetSize())
				assert.Equal(t, tc.want.Time, tc.payload.GetTime())
				assert.Equal(t, tc.want.Id, tc.payload.GetId())
				assert.Equal(t, tc.want.Padding, tc.payload.GetPadding())
			} else {
				var nilPayload *Payload
				assert.Equal(t, tc.want.Connections, nilPayload.GetConnections())
				assert.Equal(t, tc.want.Rate, nilPayload.GetRate())
				assert.Equal(t, tc.want.Size, nilPayload.GetSize())
				assert.Equal(t, tc.want.Time, nilPayload.GetTime())
				assert.Equal(t, tc.want.Id, nilPayload.GetId())
				assert.Equal(t, tc.want.Padding, nilPayload.GetPadding())
			}
		})
	}
}

func TestPayload_Marshaling(t *testing.T) {
	original := &Payload{
		Connections: 100,
		Rate:       1000,
		Size:       1024,
		Time:       timestamppb.New(time.Now().UTC()),
		Id:         []byte("test-id"),
		Padding:    []byte("test-padding"),
	}

	t.Run("marshal and unmarshal", func(t *testing.T) {
		data, err := proto.Marshal(original)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		decoded := &Payload{}
		err = proto.Unmarshal(data, decoded)
		require.NoError(t, err)

		assert.Equal(t, original.Connections, decoded.Connections)
		assert.Equal(t, original.Rate, decoded.Rate)
		assert.Equal(t, original.Size, decoded.Size)
		assert.Equal(t, original.Time.AsTime().Unix(), decoded.Time.AsTime().Unix())
		assert.Equal(t, original.Id, decoded.Id)
		assert.Equal(t, original.Padding, decoded.Padding)
	})

	t.Run("marshal and unmarshal empty payload", func(t *testing.T) {
		empty := &Payload{}
		data, err := proto.Marshal(empty)
		require.NoError(t, err)

		decoded := &Payload{}
		err = proto.Unmarshal(data, decoded)
		require.NoError(t, err)

		assert.Equal(t, empty.Connections, decoded.Connections)
		assert.Equal(t, empty.Rate, decoded.Rate)
		assert.Equal(t, empty.Size, decoded.Size)
		assert.Equal(t, empty.Time, decoded.Time)
		assert.Equal(t, empty.Id, decoded.Id)
		assert.Equal(t, empty.Padding, decoded.Padding)
	})
}

func BenchmarkPayload_Marshal(b *testing.B) {
	payload := &Payload{
		Connections: 100,
		Rate:       1000,
		Size:       1024,
		Time:       timestamppb.Now(),
		Id:         []byte("test-id"),
		Padding:    []byte("test-padding"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := proto.Marshal(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPayload_Unmarshal(b *testing.B) {
	payload := &Payload{
		Connections: 100,
		Rate:       1000,
		Size:       1024,
		Time:       timestamppb.Now(),
		Id:         []byte("test-id"),
		Padding:    []byte("test-padding"),
	}
	data, err := proto.Marshal(payload)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst := &Payload{}
		if err := proto.Unmarshal(data, dst); err != nil {
			b.Fatal(err)
		}
	}
}
