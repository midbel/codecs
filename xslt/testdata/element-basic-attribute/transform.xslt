<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<root>
			<item>
				<xsl:value-of select="/root/item"/>
			</item>
			<build-with>
				<xsl:attribute name="tool" select="'angle'"/> 
			</build-with>
		</root>
	</xsl:template>
</xsl:stylesheet>