package image

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
	"github.com/fernandobarroso/microservices/operational-workers/internal/common/utils"
)

// ImageProcessor handles image processing messages
type ImageProcessor struct {
	metrics *utils.ProcessorMetrics
}

// NewImageProcessor creates a new image processor
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		metrics: utils.NewProcessorMetrics("image"),
	}
}

// Process processes an image message
func (p *ImageProcessor) Process(ctx context.Context, msg *queue.Message) error {
	// Start metrics tracking
	timer := p.metrics.StartTimer()
	defer timer.ObserveDuration()
	p.metrics.RecordProcessingStart()

	// Convert to image message
	imageMsg, err := NewImageMessage(msg)
	if err != nil {
		p.metrics.RecordProcessingError()
		return fmt.Errorf("failed to parse image message: %w", err)
	}

	// Validate the message
	if err := p.Validate(msg); err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	// Process based on processing type
	switch imageMsg.Payload.ProcessingType {
	case ProcessingTypeResize:
		err = p.processImageResize(ctx, imageMsg)
	case ProcessingTypeFilter:
		err = p.processImageFilter(ctx, imageMsg)
	case ProcessingTypeAnalyze:
		err = p.processImageAnalyze(ctx, imageMsg)
	default:
		p.metrics.RecordProcessingError()
		return ErrInvalidProcessingType
	}

	if err != nil {
		p.metrics.RecordProcessingError()
		return err
	}

	p.metrics.RecordProcessingSuccess()
	return nil
}

// Validate validates the message
func (p *ImageProcessor) Validate(msg *queue.Message) error {
	imageMsg, err := NewImageMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to parse message for validation: %w", err)
	}
	return imageMsg.Validate()
}

// Type returns the processor type
func (p *ImageProcessor) Type() string {
	return "image"
}

// HandleError handles processing errors
func (p *ImageProcessor) HandleError(ctx context.Context, msg *queue.Message, err error) error {
	log.Printf("Image processing error: %v", err)

	// Could implement retry logic here
	// For now, just return the error which will cause message requeue
	return err
}

// processImageResize simulates calling Python image resize service
func (p *ImageProcessor) processImageResize(ctx context.Context, msg *ImageMessage) error {
	log.Printf("🖼️ Processing RESIZE for image %s (Priority: %s)",
		msg.Payload.ImageURL, msg.Payload.Priority)

	// Extract resize parameters
	width, _ := msg.Payload.Parameters["width"].(float64)
	height, _ := msg.Payload.Parameters["height"].(float64)

	log.Printf("📐 Resize parameters: %dx%d", int(width), int(height))

	// Simulate calling Python container: image-resize-service:latest
	containerName := "image-resize-service:latest"
	return p.callPythonContainer(ctx, containerName, msg)
}

// processImageFilter simulates calling Python image filter service
func (p *ImageProcessor) processImageFilter(ctx context.Context, msg *ImageMessage) error {
	log.Printf("🖼️ Processing FILTER for image %s (Priority: %s)",
		msg.Payload.ImageURL, msg.Payload.Priority)

	// Extract filter parameters
	filterType, _ := msg.Payload.Parameters["filter_type"].(string)
	intensity, _ := msg.Payload.Parameters["intensity"].(float64)

	log.Printf("🎨 Filter parameters: type=%s, intensity=%.2f", filterType, intensity)

	// Simulate calling Python container: image-filter-service:latest
	containerName := "image-filter-service:latest"
	return p.callPythonContainer(ctx, containerName, msg)
}

// processImageAnalyze simulates calling Python image analysis service
func (p *ImageProcessor) processImageAnalyze(ctx context.Context, msg *ImageMessage) error {
	log.Printf("🖼️ Processing ANALYZE for image %s (Priority: %s)",
		msg.Payload.ImageURL, msg.Payload.Priority)

	// Extract analysis parameters
	analysisType, _ := msg.Payload.Parameters["analysis_type"].(string)

	log.Printf("🔍 Analysis parameters: type=%s", analysisType)

	// Simulate calling Python container: image-analyze-service:latest
	containerName := "image-analyze-service:latest"
	return p.callPythonContainer(ctx, containerName, msg)
}

// callPythonContainer simulates calling a Python microservice container
func (p *ImageProcessor) callPythonContainer(ctx context.Context, containerName string, msg *ImageMessage) error {
	log.Printf("🐍 Calling Python container: %s", containerName)
	log.Printf("📋 Request payload: ImageURL=%s, Parameters=%v",
		msg.Payload.ImageURL, msg.Payload.Parameters)

	// Get expected processing time based on message type and priority
	processingTime := msg.GetExpectedProcessingTime()

	// Create timeout context from message timeout or use processing time
	timeoutDuration := time.Duration(msg.Payload.TimeoutSeconds) * time.Second
	if processingTime < timeoutDuration {
		timeoutDuration = processingTime
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	// Simulate the Python container processing
	select {
	case <-timeoutCtx.Done():
		if timeoutCtx.Err() == context.DeadlineExceeded {
			log.Printf("⏰ Python container call timed out after %v", timeoutDuration)
			return fmt.Errorf("python container call timed out: %w", timeoutCtx.Err())
		}
		return timeoutCtx.Err()
	case <-time.After(processingTime):
		// Simulate successful processing
		log.Printf("✅ Python container %s completed successfully", containerName)

		// Simulate callback if provided
		if msg.Payload.CallbackURL != "" {
			log.Printf("📞 Sending callback to: %s", msg.Payload.CallbackURL)
			// In real implementation, this would make HTTP POST request
		}

		return nil
	}
}
