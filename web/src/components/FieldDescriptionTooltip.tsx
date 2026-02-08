import { useRef, useState } from 'react';

// Smart tooltip that flips position based on available space
export default function FieldDescriptionTooltip({ description }: { description?: string }) {
  const iconRef = useRef<HTMLSpanElement>(null);
  const [showBelow, setShowBelow] = useState(false);
  const [showLeft, setShowLeft] = useState(false);
  const [isVisible, setIsVisible] = useState(false);
  const [position, setPosition] = useState({ top: 0, left: 0, right: 0 });

  // Guard clause - don't render if no description
  if (!description) {
    return null;
  }

  const updatePosition = () => {
    if (iconRef.current) {
      const rect = iconRef.current.getBoundingClientRect();
      // If within 120px of top, show below
      const below = rect.top < 120;
      setShowBelow(below);

      // Check if there's room to the right (256px for max-w-64)
      const tooltipWidth = 256;
      const roomOnRight = rect.right + tooltipWidth < window.innerWidth;
      setShowLeft(!roomOnRight);

      // Calculate position for fixed positioning
      setPosition({
        top: below ? rect.bottom + 4 : rect.top - 4, // 4px gap
        left: rect.right + 4, // 4px gap to the right of icon
        right: window.innerWidth - rect.left + 4, // 4px gap to the left of icon
      });
    }
  };

  const handleMouseEnter = () => {
    updatePosition();
    setIsVisible(true);
  };

  const handleMouseLeave = () => {
    setIsVisible(false);
  };

  return (
    <>
      <span
        ref={iconRef}
        className="text-gray-400 dark:text-gray-500 flex-shrink-0 ml-1.5"
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </span>
      {isVisible && (
        <span
          style={{
            top: showBelow ? position.top : 'auto',
            bottom: showBelow ? 'auto' : window.innerHeight - position.top,
            left: showLeft ? 'auto' : position.left,
            right: showLeft ? position.right : 'auto',
          }}
          className="fixed px-2 py-1 text-xs text-white bg-gray-800 dark:bg-gray-900 rounded shadow-lg whitespace-pre-wrap pointer-events-none z-50 max-w-64"
        >
          {description}
        </span>
      )}
    </>
  );
}
